#include <stdio.h>
#include <stdlib.h>
#include "machine.h"

int main(int argc, char *argv[]) {
  struct cpuid_out res;

  // Logic below from https://github.com/intel/intel-cmt-cat/blob/master/lib/cap.c
  lcpuid(0x7, 0x0, &res);
  if (!(res.ebx & (1 << 12))) {
    return EXIT_FAILURE;
  }
  // check for overall monitoring capability first
  lcpuid(0xf, 0x0, &res);
  if (!(res.edx & (1 << 1))) {
    return EXIT_FAILURE;
  }
  // check for more detailed capability, MBM monitoring
  lcpuid(0xf, 0x1, &res);
  if ((res.edx & (3 << 1)) != (3 << 1)) {
    return EXIT_FAILURE;
  }

  // If we are here, then MBM cache monitoring capability is available. 
  return EXIT_SUCCESS;
}
