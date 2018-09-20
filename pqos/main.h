/*
 * BSD LICENSE
 *
 * Copyright(c) 2014-2017 Intel Corporation. All rights reserved.
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

/**
 * @brief Platform QoS utility - main module
 */

#ifndef __MAIN_H__
#define __MAIN_H__

#ifdef DEBUG
#include <assert.h>
#endif

#ifdef __cplusplus
extern "C" {
#endif

/**
 * Macros
 */
#ifndef DIM
#define DIM(x) (sizeof(x)/sizeof(x[0]))
#endif
#ifndef MAX
/**
 * Macro to return the maximum of two numbers
 */
#define MAX(a, b) ({ \
        typeof(a) _a = (a); \
        typeof(b) _b = (b); \
        _a > _b ? _a : _b; \
})
#endif /* !MAX */

#ifdef DEBUG
#define ASSERT assert
#else
#define ASSERT(x)
#endif

#define UNUSED_ARG(_x) ((void)(_x))

#define PQOS_MAX_SOCKETS      8
#define PQOS_MAX_L2IDS        32
#define PQOS_MAX_SOCKET_CORES 64
#define PQOS_MAX_CORES        (PQOS_MAX_SOCKET_CORES*PQOS_MAX_SOCKETS)

/**
 * Maintains alloc option - allocate cores or task id's
 */
extern int alloc_pid_flag;

/**
 * Selected library interface
 */
extern int sel_interface;

/**
 * @brief Converts string into 64-bit unsigned number.
 *
 * Numbers can be in decimal or hexadecimal format.
 *
 * On error, this functions causes process to exit with FAILURE code.
 *
 * @param s string to be converted into 64-bit unsigned number
 *
 * @return Numeric value of the string representing the number
 */
uint64_t
strtouint64(const char *s);

/**
 * @brief Converts string of characters representing list of
 *        numbers into table of numbers.
 *
 * Allowed formats are:
 *     0,1,2,3
 *     0-10,20-18
 *     1,3,5-8,10,0x10-12
 *
 * Numbers can be in decimal or hexadecimal format.
 *
 * On error, this functions causes process to exit with FAILURE code.
 *
 * @param s string representing list of unsigned numbers.
 * @param tab table to put converted numeric values into
 * @param max maximum number of elements that \a tab can accommodate
 *
 * @return Number of elements placed into \a tab
 */
unsigned
strlisttotab(char *s, uint64_t *tab, const unsigned max);

/**
 * @brief Common function to handle string parsing errors
 *
 * On error, this function causes process to exit with FAILURE code.
 *
 * @param arg string that caused error when parsing
 * @param note context and information about encountered error
 */
void parse_error(const char *arg, const char *note) __attribute__ ((noreturn));

/**
 * @brief Duplicates \a arg and stores at \a sel
 *
 * @param sel place to store duplicate of \a arg
 * @param arg string passed through command line option
 */
void selfn_strdup(char **sel, const char *arg);


#ifdef __cplusplus
}
#endif

#endif /* __MAIN_H__ */
