#include <stdio.h>
#include <stdlib.h>
#include "machine.h"

int main(int argc, char *argv[]) {
  struct cpuid_out res;

  // Logic below from https://github.com/intel/intel-cmt-cat/blob/master/lib/cap.c
  lcpuid(0x7, 0x0, &res);
  if (!(res.ebx & (1 << 15))) {
    return EXIT_FAILURE;
  }
  else {
    lcpuid(0x10, 0x0, &res);
    if (!(res.ebx & (1 << 3))) {
      return EXIT_FAILURE;
    }
  }

  // If we are here, then Memory Bandwidth Allocation capability is available.
  return EXIT_SUCCESS;
}
