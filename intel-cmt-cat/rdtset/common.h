/*
 *   BSD LICENSE
 *
 *   Copyright(c) 2016-2017 Intel Corporation. All rights reserved.
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

#ifndef _COMMON_H
#define _COMMON_H

#include <stdint.h>

#include <pqos.h>

#include "cpu.h"

#ifdef __cplusplus
extern "C" {
#endif

#define RDT_MAX_SOCKETS 8
#define RDT_MAX_L2IDS   32
#define RDT_MAX_PIDS    128

#ifndef MIN
/**
 * Macro to return the minimum of two numbers
 */
#define MIN(a, b) ({ \
	typeof(a) _a = (a); \
	typeof(b) _b = (b); \
	_a < _b ? _a : _b; \
})
#endif /* !MIN */

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

#ifndef DIM
#define DIM(x) (sizeof(x)/sizeof(x[0]))
#endif /* !DIM */

#ifdef __FreeBSD__
/* Fix for "undefined reference to '__bitcountl'" */
#ifndef __bitcountl
#define __bitcountl(x) __builtin_popcountl((unsigned long)(x))
#endif /* !__bitcountl(x) */
#endif /* __FreeBSD__ */

struct rdt_cfg {
	enum pqos_cap_type type;
	union {
		struct pqos_l2ca *l2;
		struct pqos_l3ca *l3;
		struct pqos_mba *mba;
		void *generic_ptr;
	} u;
};

/**
 * @brief Creates \a rdt_cfg struct from \a pqos_l2ca struct
 *
 * @param [in] l2 L2 CAT class configuration
 *
 * @return rdt_cfg struct
 */
static inline struct rdt_cfg wrap_l2ca(struct pqos_l2ca *l2)
{
	struct rdt_cfg result;

	result.type = PQOS_CAP_TYPE_L2CA;
	result.u.l2 = l2;
	return result;
}

/**
 * @brief Creates \a rdt_cfg struct from \a pqos_l3ca struct
 *
 * @param [in] l3 L3 CAT class configuration
 *
 * @return rdt_cfg struct
 */
static inline struct rdt_cfg wrap_l3ca(struct pqos_l3ca *l3)
{
	struct rdt_cfg result;

	result.type = PQOS_CAP_TYPE_L3CA;
	result.u.l3 = l3;
	return result;
}

/**
 * @brief Creates \a rdt_cfg struct from \a pqos_mba struct
 *
 * @param [in] mba MBA class configuration
 *
 * @return rdt_cfg struct
 */
static inline struct rdt_cfg wrap_mba(struct pqos_mba *mba)
{
	struct rdt_cfg result;

	result.type = PQOS_CAP_TYPE_MBA;
	result.u.mba = mba;
	return result;
}

struct rdt_config {
	cpu_set_t cpumask;	/**< CPUs bitmask */
	struct pqos_l3ca l3;	/**< L3 configuration */
	struct pqos_l2ca l2;	/**< L2 configuration */
	struct pqos_mba mba;	/**< MBA configuretion */
        int pid_cfg;            /**< associate PIDs to this cfg */
};

/* rdtset command line configuration structure */
struct rdtset {
	pid_t pids[RDT_MAX_PIDS];	/**< process ID table */
        unsigned pid_count;             /**< Num of PIDs selected */
	struct rdt_config config[CPU_SETSIZE];	/**< RDT configuration */
	unsigned config_count;		/**< Num of RDT config entries */
	cpu_set_t cpu_aff_cpuset;	/**< CPU affinity configuration */
	cpu_set_t reset_cpuset;		/**< List of CPUs to reset COS assoc */
	unsigned sudo_keep:1,		/**< don't drop elevated privileges */
		 verbose:1,		/**< be verbose */
		 command:1;		/**< command to be executed detected */
	int interface;                  /**< pqos interface to use */
};

struct rdtset g_cfg;

/**
 * @brief Parse CPU set string
 *
 * @note Parse elem, the elem could be single number/range or group
 *       1) A single number elem, it's just a simple digit. e.g. 9
 *       2) A single range elem, two digits with a '-' between. e.g. 2-6
 *       3) A group elem, combines multiple 1) or 2) with e.g 0,2-4,6
 *       Within group elem, '-' used for a range separator;
 *       ',' used for a single number.
 *
 * @param [in] cpustr string representation of a cpu set
 * @param [in] cpustr_len len of \a cpustr
 * @param [out] cpuset parsed cpuset
 *
 * @return number of parsed characters on success
 * @retval -ERRNO on error
 */
int str_to_cpuset(const char *cpustr, const unsigned cpustr_len,
		cpu_set_t *cpuset);

/**
 * @brief Converts CPU set (cpu_set_t) to string
 *
 * @param [out] cpustr output string
 * @param [in] cpustr_len max output string len
 * @param [in] cpumask input cpuset
 */
void cpuset_to_str(char *cpustr, const unsigned cpustr_len,
		const cpu_set_t *cpumask);

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

#ifdef __cplusplus
}
#endif

#endif /* _COMMON_H */
