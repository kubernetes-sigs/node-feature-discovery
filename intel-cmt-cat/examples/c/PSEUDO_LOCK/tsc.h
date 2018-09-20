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

#include <stdint.h>

#ifndef __TSC_H__
#define __TSC_H__

#ifdef __cplusplus
extern "C" {
#endif

/**
 * TSC profile structure
 */
struct tsc_prof {
        uint64_t clk_start;           /**< start TSC of an iteration */
        uint64_t clk_avgc;            /**< count to calculate an average */
        double clk_min;               /**< min cycle cost recorded */
        double clk_max;               /**< max cycle cost recorded */
        double clk_avg;               /**< cumulative sum to
                                         calculate an average */
        double clk_result;            /**< avg cycles cost */
        double cost;                  /**< cost of measurement */
        char name[128];
};

/**
 * @brief Get TSC value for the start of measured block of code
 *
 * This function prevents out of order execution before reading TSC.
 * LFENCE instruction is used for it:
 *   - no OOO
 *   - load buffers are empty after lfence
 *   - no deliberate restrictions on store buffers, some stores may drain though
 * Another options to prevent OOO are:
 *   - cpuid; affects LB and SB (both get emptied)
 *   - forced branch miss-prediction; no effect on LB or SB but
 *     loads/stores may drain
 * When measured code has very high cycle cost preventing OOO
 * may not be required and RDTSCP instruction may be enough.
 *
 * @return TSC value
 */
static __attribute__((always_inline)) inline uint64_t __tsc_start(void)
{
        uint32_t cycles_high, cycles_low;

#ifdef __x86_64__
        asm volatile("lfence\n\t"
                     "rdtscp\n\t"
                     "mov %%edx, %0\n\t"
                     "mov %%eax, %1\n\t"
                     : "=r" (cycles_high), "=r" (cycles_low)
                     : : "%rax", "%rdx");
#else
        asm volatile("lfence\n\t"
                     "rdtscp\n\t"
                     "mov %%edx, %0\n\t"
                     "mov %%eax, %1\n\t"
                     : "=r" (cycles_high), "=r" (cycles_low)
                     : : "%eax", "%edx");
#endif
        return(((uint64_t)cycles_high << 32) | cycles_low);
}

/**
 * @brief Get TSC value for the end of measured block of code
 *
 * No OOO prevention required. RDTSCP is used here which makes sure
 * all previous instructions retire before reading TSC.
 *
 * @return TSC value
 */
static __attribute__((always_inline)) inline uint64_t __tsc_end(void)
{
        uint32_t cycles_high, cycles_low;

#ifdef __x86_64__
        asm volatile("rdtscp\n\t"
                     "mov %%edx, %0\n\t"
                     "mov %%eax, %1\n\t"
                     : "=r" (cycles_high), "=r" (cycles_low)
                     : : "%rax", "%rdx");
#else
        asm volatile("rdtscp\n\t"
                     "mov %%edx, %0\n\t"
                     "mov %%eax, %1\n\t"
                     : "=r" (cycles_high), "=r" (cycles_low)
                     : : "%eax", "%edx");
#endif
        return ((uint64_t)cycles_high << 32) | cycles_low;
}

/**
 * @brief Starts cycle measurement of code iteration
 *
 * tsc_start() and tsc_end() or tsc_end_ex() can be called multiple times.
 *
 * @param p pointer to TSC profile structure
 */
static __attribute__((always_inline)) inline void
tsc_start(struct tsc_prof *p)
{
        p->clk_start = __tsc_start();
}

/**
 * @brief Stops cycle measurement of code iteration
 *
 * @param p pointer to TSC profile structure
 * @param inc number of items processed within the iteration.
 *        This allows code to calculate average cycle cost per work item even
 *        though number of code iterations may be different.
 * @param clk_start start TSC value. This is useful when using one
 *        start TSC reading for multiple different TSC profiles.
 */
static __attribute__((always_inline)) inline void
tsc_end_ex(struct tsc_prof *p, const unsigned inc, const uint64_t clk_start)
{
        double clk_diff = (double) (__tsc_end() - clk_start);

        p->clk_avgc += inc;
        p->clk_avg += (clk_diff - p->cost);

        clk_diff = clk_diff / (double) inc;

        if (clk_diff < p->clk_min)
                p->clk_min = clk_diff;
        if (clk_diff > p->clk_max)
                p->clk_max = clk_diff;
}

/**
 * @brief Stops cycle measurement of code iteration
 *
 * @param p pointer to TSC profile structure
 * @param inc number of items processed within the iteration
 */
static __attribute__((always_inline)) inline void
tsc_end(struct tsc_prof *p, const unsigned inc)
{
        tsc_end_ex(p, inc, p->clk_start);
}

/**
 * @brief Calculates an average cycle cost per item
 *
 * Calculated average cycle cost is also stored in TSC profile structure.
 *
 * @param p pointer to TSC profile structure
 *
 * @return Calculated average cycle cost per work item
 * @retval NAN if no code measurement done so far
 */
static __attribute__((always_inline)) inline
double tsc_get_avg(struct tsc_prof *p)
{
        double avg_c = 0.0;

        if (p->clk_avgc > 0)
                avg_c = (p->clk_avg / ((double) p->clk_avgc));
        p->clk_result = avg_c;
        return avg_c;
}

/**
 * @brief Initializes TSC profile structure
 *
 * @param p pointer to TSC profile structure
 * @param name string describing the measurement in printf() format
 * @param ... variadic arguments depending on \a name format
 */
void tsc_init(struct tsc_prof *p, const char *name, ...);

/**
 * @brief Prints measured TSC profile data
 *
 * @param p pointer to TSC profile structure
 */
void tsc_print(struct tsc_prof *p);

#ifdef __cplusplus
}
#endif

#endif /* __TSC_H__ */
