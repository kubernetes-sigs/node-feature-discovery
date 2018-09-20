/*
 *   BSD LICENSE
 *
 *   Copyright(c) 2016 Intel Corporation. All rights reserved.
 *   All rights reserved.
 *
 *   Redistribution and use in source and binary forms, with or without
 *   modification, are permitted provided that the following conditions
 *   are met:
 *
 *     * Redistributions of source code must retain the above copyright
 *       notice, this list of conditions and the following disclaimer.
 *     * Redistributions in binary form must reproduce the above copyright
 *       notice, this list of conditions and the following disclaimer in
 *       the documentation and/or other materials provided with the
 *       distribution.
 *     * Neither the name of Intel Corporation nor the names of its
 *       contributors may be used to endorse or promote products derived
 *       from this software without specific prior written permission.
 *
 *   THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 *   "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 *   LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 *   A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 *   OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 *   SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 *   LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 *   DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 *   THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 *   (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 *   OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

#ifndef _CPU_H
#define _CPU_H

#ifdef __linux__
#include <sched.h>
#endif
#ifdef __FreeBSD__
#include <sys/param.h>
#include <sys/cpuset.h>
#endif

#ifdef __cplusplus
extern "C" {
#endif

#ifdef __FreeBSD__
typedef cpuset_t cpu_set_t;
#ifndef CPU_COUNT
static inline int
CPU_COUNT(const cpu_set_t *set)
{
	int i = 0, count = 0;

	for (i = 0; i < CPU_SETSIZE; i++)
		if (CPU_ISSET(i, set))
			count++;

	return count;
}
#endif /* !CPU_COUNT */
#endif /* __FreeBSD__ */

/**
 * @brief Parse -c/--cpu params
 *
 * @param [in] cpu params string
 *
 * @return status
 * @retval 0 on success
 * @retval negative on error (-errno)
 */
int parse_cpu(const char *cpu);

/**
 * @brief Set process CPU affinity
 *
 * @param [in] pid pid of process (0 for current)
 *
 * @return status
 * @retval 0 on success
 * @retval -1 on error
 */
int set_affinity(pid_t pid);

/**
 * @brief Print parsed -c/--cpu config
 */
void print_cmd_line_cpu_config(void);

#ifdef __cplusplus
}
#endif

#endif /* _CPU_H */
