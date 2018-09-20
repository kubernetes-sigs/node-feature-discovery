/*
 * BSD LICENSE
 *
 * Copyright(c) 2016 Intel Corporation. All rights reserved.
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 *
 *   * Redistributions of source code must retain the above copyright
 *     notice, this list of conditions and the following disclaimer.
 *   * Redistributions in binary form must reproduce the above copyright
 *     notice, this list of conditions and the following disclaimer in
 *     the documentation and/or other materials provided with the
 *     distribution.
 *   * Neither the name of Intel Corporation nor the names of its
 *     contributors may be used to endorse or promote products derived
 *     from this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 * LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 * A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 * OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 * SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 * LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

#include <stdio.h>   /* vsnprintf() */
#include <stdarg.h>  /* va_start(), va_end() */
#include <limits.h>  /* ULONG_MAX */
#include <string.h>  /* memset() */
#include "tsc.h"

static const double __measurement_cost = 0;

void tsc_init(struct tsc_prof *p, const char *name, ...)
{
        va_list ap;

        va_start(ap, name);
        memset(p->name, 0, sizeof(p->name));
        vsnprintf(p->name, sizeof(p->name) - 1, name, ap);
        va_end(ap);

        p->clk_avg = 0;
        p->clk_avgc = 0;
        p->clk_result = 0.0;
        p->clk_max = 0.0;
        p->clk_min = (double) ULONG_MAX;
        p->cost = __measurement_cost;
}

void tsc_print(struct tsc_prof *p)
{
        tsc_get_avg(p);

        printf("[%s] work items %llu; cycles per work item: "
               "avg=%.3f min=%.3f max=%.3f jitter=%.3f\n",
               p->name, (unsigned long long)p->clk_avgc,
               p->clk_result, p->clk_min, p->clk_max, p->clk_max - p->clk_min);
}
