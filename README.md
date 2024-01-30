# postings-analyzer

This program analyzes lists of postings in a block and outputs statistics about it.

Outputs statistics about:

- Raw postings length of each pair
- That list converted into a roaring bitmap of each pair
- That list converted into a roaring bitmap of each pair after RLE analysis
- That list converted into S4-BP128-D4

Example run:


```bash
go run main.go /home/giedrius/dev/prometheus/data 01HMR1ZMB1RP9554XT634AV5VG
```


For `/home/giedrius/dev/go-bp/originalimpl`, please compile `originalimpl.cpp` with https://github.com/lemire/SIMDCompressionAndIntersection/tree/master.

## Results

Ran this program on a few big blocks. Our workload contains lots of churn every 15 minutes.

```
Number of label name/value pairs: 22941
-----RAW-----
50th by posting list size in bytes: 1067.8972602739725
75th by posting list size in bytes: 15487.599999999997
90th by posting list size in bytes: 150934.05432098766
99th by posting list size in bytes: 806052.6259340661
Total sum 3395576308
-----Roaring-----
50th by posting list size in bytes: 836.8398711176053
75th by posting list size in bytes: 8097.92
90th by posting list size in bytes: 113436.20275011855
99th by posting list size in bytes: 450637.2832653062
Total sum 1830630574
-----Roaring RLE-----
50th by posting list size in bytes: 1
75th by posting list size in bytes: 1
90th by posting list size in bytes: 1
99th by posting list size in bytes: 1
Total sum 26414
-----S4BP128D4-----
50th by posting list size in bytes: 741.406914303466
75th by posting list size in bytes: 5451.680000000001
90th by posting list size in bytes: 83056.87843137271
99th by posting list size in bytes: 369353.9266666667
Total sum 1021119780
```

```
Number of label name/value pairs: 23342
-----RAW-----
50th by posting list size in bytes: 1123.7475345167654
75th by posting list size in bytes: 12230.847727272729
90th by posting list size in bytes: 160590.6515235457
99th by posting list size in bytes: 946400.5066666654
Total sum 3921333720
-----Roaring-----
50th by posting list size in bytes: 632.0791893291894
75th by posting list size in bytes: 6460.864942528736
90th by posting list size in bytes: 119373.147518797
99th by posting list size in bytes: 526809.6984615371
Total sum 2091835366
-----Roaring RLE-----
50th by posting list size in bytes: 1
75th by posting list size in bytes: 1
90th by posting list size in bytes: 1
99th by posting list size in bytes: 1
Total sum 26726
-----S4BP128D4-----
50th by posting list size in bytes: 643.5959595959595
75th by posting list size in bytes: 4558.959522657636
90th by posting list size in bytes: 86933.56119711074
99th by posting list size in bytes: 396866.2066666665
Total sum 1159584372
```

Smaller churn index but with more label name/value pairs:

```
Number of label name/value pairs: 111756
-----RAW-----
50th by posting list size in bytes: 96.56487721744524
75th by posting list size in bytes: 739.0367738671047
90th by posting list size in bytes: 5164
99th by posting list size in bytes: 43246.93661538464
Total sum 1105118676
-----Roaring-----
50th by posting list size in bytes: 110.74389207082254
75th by posting list size in bytes: 1298
90th by posting list size in bytes: 3620
99th by posting list size in bytes: 29064.842226613975
Total sum 497867898
-----Roaring RLE-----
50th by posting list size in bytes: 1
75th by posting list size in bytes: 1
90th by posting list size in bytes: 1
99th by posting list size in bytes: 1
Total sum 115764
-----S4BP128D4-----
50th by posting list size in bytes: 68
75th by posting list size in bytes: 617.7637927590845
90th by posting list size in bytes: 3668
99th by posting list size in bytes: 25269.08812127622
Total sum 367533928
```

Seems like at least in these cases RB doesn't do RLE because the series IDs in postings have a bit of gap between them. RLE automatically uses RLE if it would benefit from that. It is possible to imagine a setup where series IDs are constantly increasing. For example, only one target is being scraped with millions of series and no churn.

S4-BP128-D4 is ~2x smaller than RB however RB offers great intersection speed. Quick intersection and merging operations come from the fact that it is enough to bitwise AND/OR the respective containers to get a result.

 https://arxiv.org/pdf/1401.6399.pdf paper offers a way of implementing SIMD intersection using regular arrays however that is not really possible to implement in Go (from my tests) in a performant way because Go doesn't inline functions written in assembler. See: https://dave.cheney.net/2019/08/20/go-compiler-intrinsics. Also see https://groups.google.com/g/golang-nuts/c/yVOfeHYCIT4. We need inlining because the `index.Postings` interface has `Next() bool` and `At() storage.SeriesRef` methods that needs to be constantly called to get the next item in a postings list. I'll try to open source my implementation.

It's possible to use SIMD with RB too (but the Go implementation uses it for a very small portion) so maybe time should be spent optimizing the Go RoaringBitmaps implementation for now until Go implements intrinsics or something similar. Also, there's a mature RB Go implementation so we could use that to unblock 64 bit postings in Prometheus.

Interestingly enough, RB inside itself has a galloping intersection algorithm onesidedgallopingintersect2by2 https://github.com/RoaringBitmap/roaring/blob/0d5af7578725804c91db5cea10b0504baa7b0a2b/setutil.go#L479 that the paper also talks about.

Pros of S4-BP128-D4:

- Better compression ratio in tests
- SIMD oriented data layout

Cons of S4-BP128-D4:

- Basic intersection algorithm
- No 64 bit implementation

Pros of roaring bitmaps:

- Mature Go implementation
- Many implementations in other languages
- Supports 64 bits
* Has RLE support which might be useful in some (rare?) setups

Cons of roaring bitmaps:

- Worse compression ratio in tests
