// Original from https://github.com/lemire/SIMDCompressionAndIntersection/blob/master/example.cpp.
//
// A simple example to get you started with the library.
// You can compile and run this example like so:
//
//   make example
//   ./example
//
//  Warning: If your compiler does not fully support C++11, some of
//  this example may require changes.
//

#include "codecfactory.h"
#include "intersection.h"
#include <iterator>

using namespace SIMDCompressionLib;

template<typename T>
std::ostream & operator<<(std::ostream & os, std::vector<T> vec)
{
    os<<"{ ";
    std::copy(vec.begin(), vec.end(), std::ostream_iterator<T>(os, " "));
    os<<"}";
    return os;
}

int main(int argc, char *argv[]) {
  // We pick a CODEC
  IntegerCODEC &codec = *CODECFactory::getFromName("s4-bp128-d4");
  // could use others, e.g., frameofreference, ibp32, maskedvbyte, s4-bp128-d1, s4-bp128-d2, s4-bp128-d4, s4-bp128-dm, simdframeofreference, streamvbyte
  //
  // Note that some codecs compute the differential coding in-place, thus modifying part of the input, replacing it with a differentially coded version:
  //  bp32, fastpfor, s4-bp128-d1-ni, s4-bp128-d2-ni, s4-bp128-d4-ni, s4-bp128-dm-ni, s4-fastpfor-d1, s4-fastpfor-d2, s4-fastpfor-d4, s4-fastpfor-dm
  // Other codecs do the differential coding "in passing", such as
  // for, frameofreference, ibp32, maskedvbyte, s4-bp128-d1, s4-bp128-d2, s4-bp128-d4, s4-bp128-dm, simdframeofreference, streamvbyte, varint, varintg8iu, varintgb,  vbyte
  //


  ////////////
  //
  // create a container with some integers in it
  //
  // We need the integers to be in sorted order.
  //
  // (Note: You don't need to use a vector.)
  //

  vector<uint32_t> mydata;
  std::ifstream infile(argv[1]);
  uint32_t input;
  size_t N = 0;
  while (infile >> input) {
    mydata.push_back(input);
    N++;
  }


  std::cout << "Retrieved data:" << mydata << std::endl;


  // we make a copy
  std::vector<uint32_t> original_data(mydata);
  ///////////
  //
  // You need some "output" container. You are responsible
  // for allocating enough memory.
  //
  vector<uint32_t> compressed_output(N + 1024);
  // N+1024 should be plenty
  //
  //
  size_t compressedsize = compressed_output.size();
  codec.encodeArray(mydata.data(), mydata.size(), compressed_output.data(),
                    compressedsize);
  //
  // if desired, shrink back the array:
  compressed_output.resize(compressedsize);
  compressed_output.shrink_to_fit();

  std::ofstream wf(argv[2], std::ios::binary);
  wf.write(reinterpret_cast<char*>(compressed_output.data()), compressed_output.size() * sizeof(uint32_t));
  wf.close();

  //
  // You are done!... with the compression...
  //
  ///
  // decompressing is also easy:
  //
  vector<uint32_t> mydataback(N);
  size_t recoveredsize = mydataback.size();
  //
  codec.decodeArray(compressed_output.data(), compressed_output.size(),
                    mydataback.data(), recoveredsize);
  mydataback.resize(recoveredsize);
  //
  // That's it for compression!
  //
  if (mydataback != original_data)
    throw runtime_error("bug!");
  std::cout << "Decoded data:" << mydataback << std::endl;
}
