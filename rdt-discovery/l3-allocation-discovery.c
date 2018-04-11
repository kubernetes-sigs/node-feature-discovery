#include <stdio.h>
#include <stdlib.h>
#include "machine.h"

int main(int argc, char *argv[]) {
  struct cpuid_out res;

  // Logic below from https://github.com/intel/intel-cmt-cat/blob/master/lib/cap.c
  // TODO(balajismaniam): Implement L3 CAT detection using brand string and MSR probing if
  // not detected using cpuid
  lcpuid(0x7, 0x0, &res);
  if (!(res.ebx & (1 << 15))) {
    return EXIT_FAILURE;
  }
  else {
    lcpuid(0x10, 0x0, &res);
    if (!(res.ebx & (1 << 1))) {
      return EXIT_FAILURE;
    }
  }

  // If we are here, then L3 cache allocation capability is available.
  return EXIT_SUCCESS;
}
