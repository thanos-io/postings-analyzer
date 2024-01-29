package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/RoaringBitmap/roaring"
	"github.com/cespare/xxhash/v2"
	"github.com/influxdata/tdigest"
	"github.com/prometheus/prometheus/tsdb"
	"github.com/sourcegraph/conc/pool"
)

func compress32WithVanillaImpl(data []uint32) []byte {
	td := os.TempDir()

	f, err := os.Create(filepath.Join(td, "input.bin"))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	for _, v := range data {
		fmt.Fprintf(bw, "%d\n", v)
	}
	if err := bw.Flush(); err != nil {
		panic(err)
	}

	c := exec.Command("/home/giedrius/dev/go-bp/originalimpl", filepath.Join(td, "input.bin"), filepath.Join(td, "output.bin"))

	err = c.Run()
	if err != nil {
		panic(err)
	}

	outContent, err := os.ReadFile(filepath.Join(td, "output.bin"))
	if err != nil {
		panic(err)
	}

	return outContent
}

func openBlock(path, blockID string) (*tsdb.DBReadOnly, tsdb.BlockReader, error) {
	db, err := tsdb.OpenDBReadOnly(path, nil)
	if err != nil {
		return nil, nil, err
	}

	if blockID == "" {
		blockID, err = db.LastBlockID()
		if err != nil {
			return nil, nil, err
		}
	}

	b, err := db.Block(blockID)
	if err != nil {
		return nil, nil, err
	}

	return db, b, nil
}

type labelPair struct {
	length int

	rawSizeBytes, roaringSizeBytes, roaringRLESizeBytes, s4bp128d4SizeBytes uint64

	name, value string

	// Roaring size.
	// Roaring size with RLE.

}

type listPostings struct {
	refs []uint32
}

func (lp *listPostings) Hash() uint64 {
	h := xxhash.New()

	for _, r := range lp.refs {
		fmt.Fprintf(h, "%d", r)
	}

	return h.Sum64()
}

func main() {
	path, blockID := os.Args[1], os.Args[2]

	db, b, err := openBlock(path, blockID)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	ir, err := b.Index()
	if err != nil {
		panic(err)
	}
	defer ir.Close()

	statsMtx := sync.Mutex{}
	stats := map[uint64]labelPair{}
	var pl *pool.Pool = pool.New().WithMaxGoroutines(100)

	labelNames, err := ir.LabelNames(context.Background())
	if err != nil {
		panic(err)
	}

	sumofPostingsListRawSizes := uint64(0)
	sumofPostingsListRoaringSizes := uint64(0)
	sumofPostingsListRoaringRLESizes := uint64(0)
	sumOfPostingsListS4BP128D4Sizes := uint64(0)

	rawPostingsDistribution := tdigest.NewWithCompression(1000)
	roaringPostingsDistribution := tdigest.NewWithCompression(1000)
	roaringPostingsRLEDistribution := tdigest.NewWithCompression(1000)
	s4bp128d4Distribution := tdigest.NewWithCompression(1000)

	for _, ln := range labelNames {
		lvs, err := ir.LabelValues(context.Background(), ln)
		if err != nil {
			panic(err)
		}
		ln := ln

		for _, lv := range lvs {
			lv := lv

			p, err := ir.Postings(context.Background(), ln, lv)
			if err != nil {
				panic(err)
			}

			pl.Go(func() {
				lp := listPostings{
					refs: make([]uint32, 0),
				}
				for p.Next() {
					lp.refs = append(lp.refs, uint32(p.At()))
				}

				rb := roaring.New()
				rb.AddMany(lp.refs)

				roaringOutput, err := rb.ToBytes()
				if err != nil {
					panic(err)
				}

				rb.RunOptimize()
				if rb.HasRunCompression() {
					fmt.Println(lp.refs)
				}

				roaringOutputRLE, err := rb.ToBytes()
				if err != nil {
					panic(err)
				}

				s4bp128d4 := compress32WithVanillaImpl(lp.refs)

				statsMtx.Lock()
				stats[lp.Hash()] = labelPair{
					length:              len(lp.refs),
					rawSizeBytes:        4 + uint64(len(lp.refs))*4,
					name:                ln,
					value:               lv,
					roaringSizeBytes:    uint64(len(roaringOutput)),
					roaringRLESizeBytes: uint64(len(roaringOutputRLE)),
					s4bp128d4SizeBytes:  uint64(len(s4bp128d4)),
				}

				sumofPostingsListRawSizes += 4 + uint64(len(lp.refs))*4
				sumofPostingsListRoaringSizes += uint64(len(roaringOutput))
				sumofPostingsListRoaringRLESizes += uint64(len(roaringOutputRLE))
				sumOfPostingsListS4BP128D4Sizes += uint64(len(s4bp128d4))

				rawPostingsDistribution.Add(float64(4+uint64(len(lp.refs))*4), 1)
				roaringPostingsDistribution.Add(float64(len(roaringOutput)), 1)
				roaringPostingsRLEDistribution.Add(float64(len(roaringOutputRLE)), 1)
				s4bp128d4Distribution.Add(float64(len(s4bp128d4)), 1)

				if len(stats)%100 == 0 {
					fmt.Println("Loaded 100", ln, lv)
				}
				statsMtx.Unlock()
			})
		}
	}

	pl.Wait()

	fmt.Printf("Number of label name/value pairs: %d\n", len(stats))

	fmt.Println("-----RAW-----")
	// Compute Quantiles
	fmt.Println("50th by posting list size in bytes:", rawPostingsDistribution.Quantile(0.5))
	fmt.Println("75th by posting list size in bytes:", rawPostingsDistribution.Quantile(0.75))
	fmt.Println("90th by posting list size in bytes:", rawPostingsDistribution.Quantile(0.9))
	fmt.Println("99th by posting list size in bytes:", rawPostingsDistribution.Quantile(0.99))

	fmt.Println("Total sum", sumofPostingsListRawSizes)

	fmt.Println("-----Roaring-----")
	// Compute Quantiles
	fmt.Println("50th by posting list size in bytes:", roaringPostingsDistribution.Quantile(0.5))
	fmt.Println("75th by posting list size in bytes:", roaringPostingsDistribution.Quantile(0.75))
	fmt.Println("90th by posting list size in bytes:", roaringPostingsDistribution.Quantile(0.9))
	fmt.Println("99th by posting list size in bytes:", roaringPostingsDistribution.Quantile(0.99))

	fmt.Println("Total sum", sumofPostingsListRoaringSizes)

	fmt.Println("-----Roaring RLE-----")
	// Compute Quantiles
	fmt.Println("50th by posting list size in bytes:", roaringPostingsRLEDistribution.Quantile(0.5))
	fmt.Println("75th by posting list size in bytes:", roaringPostingsRLEDistribution.Quantile(0.75))
	fmt.Println("90th by posting list size in bytes:", roaringPostingsRLEDistribution.Quantile(0.9))
	fmt.Println("99th by posting list size in bytes:", roaringPostingsRLEDistribution.Quantile(0.99))

	fmt.Println("Total sum", sumofPostingsListRoaringRLESizes)

	fmt.Println("-----S4BP128D4-----")
	// Compute Quantiles
	fmt.Println("50th by posting list size in bytes:", s4bp128d4Distribution.Quantile(0.5))
	fmt.Println("75th by posting list size in bytes:", s4bp128d4Distribution.Quantile(0.75))
	fmt.Println("90th by posting list size in bytes:", s4bp128d4Distribution.Quantile(0.9))
	fmt.Println("99th by posting list size in bytes:", s4bp128d4Distribution.Quantile(0.99))

	fmt.Println("Total sum", sumOfPostingsListS4BP128D4Sizes)

}
