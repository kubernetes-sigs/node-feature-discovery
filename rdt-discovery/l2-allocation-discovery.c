/*
Copyright 2016 Intel Corporation
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
    printf("NOT DETECTED");
  }
  else {
    lcpuid(0x10, 0x0, &res);
    if (!(res.ebx & (1 << 2))) {
      det = 0;
      printf("NOT DETECTED");
    }
  }

  if (det)
    printf("DETECTED");

  return 0;
}
