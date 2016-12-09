#include <stdio.h>
#include <stdlib.h>
#include "machine.h"

int main(int argc, char *argv[]) {
  int ret, det=1;
  struct cpuid_out res;

  // Logic below from https://github.com/01org/intel-cmt-cat/blob/master/lib/host_cap.c
  lcpuid(0x7, 0x0, &res);
  if (!(res.ebx & (1 << 15))) {
    det = 0;
    return EXIT_FAILURE;
  }
  else {
    lcpuid(0x10, 0x0, &res);
    if (!(res.ebx & (1 << 2))) {
      det = 0;
      return EXIT_FAILURE;
    }
  }

  if (det)
    return EXIT_SUCCESS;
}
