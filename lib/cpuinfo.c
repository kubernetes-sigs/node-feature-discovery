/*
 * BSD LICENSE
 *
 * Copyright(c) 2014-2016 Intel Corporation. All rights reserved.
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
 *
 */

/**
 * @brief CPU sockets and cores enumeration module.
 */

#include <string.h>
#include <stdint.h>
#include <limits.h>
#include <errno.h>
#include <unistd.h>      /* sysconf() */
#ifdef __FreeBSD__
#include <sys/param.h>   /* sched affinity */
#include <sys/cpuset.h>  /* sched affinity */
#endif
#ifdef __linux__
#include <sched.h>       /* sched affinity */
#endif

#include "log.h"
#include "cpuinfo.h"
#include "types.h"
#include "machine.h"

/**
 * This structure will be made externally available
 * If not NULL then module is initialized.
 */
static struct pqos_cpuinfo *m_cpu = NULL;

/**
 * L2 and L3 cache info structures
 */
static struct pqos_cacheinfo m_l2;
static struct pqos_cacheinfo m_l3;

/**
 * Internal APIC information structure
 */
struct apic_info {
        uint32_t smt_mask;      /**< mask to get SMT ID */
        uint32_t smt_size;      /**< size of SMT ID mask */
        uint32_t core_mask;     /**< mask to get CORE ID */
        uint32_t core_smt_mask; /**< mask to get CORE+SMT ID */
        uint32_t pkg_mask;      /**< mask to get PACKAGE ID */
        uint32_t pkg_shift;     /**< bits to shift to get PACKAGE ID */
        uint32_t l2_shift;      /**< bits to shift to get L2 ID */
        uint32_t l3_shift;      /**< bits to shift to get L3 ID */
};

/**
 * Own typedef to simplify dealing with cpu set differences.
 */
#ifdef __FreeBSD__
typedef cpuset_t cpu_set_t; /* stick with Linux typedef */
#endif

/**
 * @brief Sets current task CPU affinity as specified by \a p_set mask
 *
 * @param [in] p_set CPU set
 *
 * @return Operation status
 * @retval 0 OK
 */
static int
set_affinity_mask(const cpu_set_t *p_set)
{
        if (p_set == NULL)
                return -EFAULT;
#ifdef __linux__
        return sched_setaffinity(0, sizeof(*p_set), p_set);
#endif
#ifdef __FreeBSD__
        return cpuset_setaffinity(CPU_LEVEL_WHICH, CPU_WHICH_TID, -1,
                                  sizeof(*p_set), p_set);
#endif
}

/**
 * @brief Sets current task CPU affinity to specified core \a id
 *
 * @param [in] id core id
 *
 * @return Operation status
 * @retval 0 OK
 */
static int
set_affinity(const int id)
{
        cpu_set_t cpuset;

        CPU_ZERO(&cpuset);
        CPU_SET(id, &cpuset);
        return set_affinity_mask(&cpuset);
}

/**
 * @brief Retrieves current task core affinity
 *
 * @param [out] p_set place to store current CPU set
 *
 * @return Operation status
 * @retval 0 OK
 */
static int
get_affinity(cpu_set_t *p_set)
{
        if (p_set == NULL)
                return -EFAULT;

        CPU_ZERO(p_set);
#ifdef __linux__
        return sched_getaffinity(0, sizeof(*p_set), p_set);
#endif
#ifdef __FreeBSD__
        return cpuset_getaffinity(CPU_LEVEL_WHICH, CPU_WHICH_TID, -1,
                                  sizeof(*p_set), p_set);
#endif
}

/**
 * @brief Discovers APICID structure information
 *
 * Uses CPUID leaf 0xB to find SMT, CORE and package APICID information.
 *
 * @param [out] apic structure to be filled in with details
 *
 * @return Operation status
 * @retval 0 OK
 * @retval -1 error
 */
static int
detect_apic_core_masks(struct apic_info *apic)
{
        int core_reported = 0;
        int thread_reported = 0;
        unsigned subleaf = 0;

        for (subleaf = 0; ; subleaf++) {
                struct cpuid_out leafB;
                unsigned level_type, level_shift;
                uint32_t mask;

                lcpuid(0xb, subleaf, &leafB);
                if (leafB.ebx == 0) /* invalid sub-leaf */
                        break;

                level_type = (leafB.ecx >> 8) & 0xff;     /* ECX bits 15:8 */
                level_shift = leafB.eax & 0x1f;           /* EAX bits 4:0 */
                mask = ~(UINT32_MAX << level_shift);

                if (level_type == 1) {
                        /* level_type 1 is for SMT  */
                        apic->smt_mask = mask;
                        apic->smt_size = level_shift;
                        thread_reported = 1;
                }
                if (level_type == 2) {
                        /* level_type 2 is for CORE */
                        apic->core_smt_mask = mask;
                        apic->pkg_shift = level_shift;
                        apic->pkg_mask = ~mask;
                        core_reported = 1;
                }
        }

        if (!thread_reported)
                return -1;

        if (core_reported) {
                apic->core_mask = apic->core_smt_mask ^ apic->smt_mask;
        } else {
                apic->core_mask = 0;
                apic->pkg_shift = apic->smt_size;
                apic->pkg_mask = ~apic->smt_mask;
        }

        return 0;
}

/**
 * @brief Calculates number of bits needed for \a n.
 *
 * This is utility function for detect_apic_cache_masks() and
 * the logic follows Table 3-18 Intel(R) SDM recommendations.
 * It finds nearest power of 2 that is not smaller than \a n
 * and returns number of bits required to encode \a n.
 *
 * @param [in] n input number to find nearest power of 2 for
 *
 * @return Nearest power of 2 not smaller than \a n
 */
static unsigned
nearest_pow2(const unsigned n)
{
        unsigned r, p;

	if (n < 2)
                return n;

        for (r = 1, p = 0; r != 0; r <<= 1, p++)
                if (r >= n)
                        break;
        return p;
}

/**
 * @brief Discovers cache APICID structure information
 *
 * Uses CPUID leaf 0x4 to find L3 and L2 cache APICID information.
 *
 * Assumes apic->pkg_shift is already set, this is in case
 * L3/LLC is not detected.
 *
 * Fills in information about L2 and L3 caches into \a m_l2 and \a m_l3
 * data structures.
 *
 * @param [out] apic structure to be filled in with details
 *
 * @return Operation status
 * @retval 0 OK
 * @retval -1 error
 */
static int
detect_apic_cache_masks(struct apic_info *apic)
{
        unsigned cache_level_shift[4] = {0, 0, 0, 0};
        unsigned subleaf = 0;

        memset(&m_l2, 0, sizeof(m_l2));
        memset(&m_l3, 0, sizeof(m_l3));

        for (subleaf = 0; ; subleaf++) {
                struct cpuid_out leaf4;
                struct pqos_cacheinfo ci;
                unsigned cache_type, cache_level, id, shift;

                lcpuid(0x4, subleaf, &leaf4);
                cache_type = leaf4.eax & 0x1f;            /* EAX bits 04:00 */
                cache_level = (leaf4.eax >> 5) & 0x7;     /* EAX bits 07:05 */
                id = (leaf4.eax >> 14) & 0xfff;           /* EAX bits 25:14 */
                shift = nearest_pow2(id + 1);

                if (cache_type == 0 || cache_type >= 4)
                        break;    /* no more caches or reserved */

                if (cache_level < DIM(cache_level_shift))
                        cache_level_shift[cache_level] = shift;

                LOG_INFO("CACHE: type %u, level %u, "
                         "max id sharing this cache %u (%u bits)\n",
                         cache_type, cache_level, id + 1, shift);

                memset(&ci, 0, sizeof(ci));
                ci.detected = 1;
                ci.num_ways = (leaf4.ebx >> 22) + 1;
                ci.num_sets = leaf4.ecx + 1;
                ci.line_size = (leaf4.ebx & 0xfff) + 1;
                ci.num_partitions = ((leaf4.ebx >> 12) & 0x3ff) + 1;
                ci.way_size = ci.num_partitions * ci.num_sets * ci.line_size;
                ci.total_size = ci.way_size * ci.num_ways;

                LOG_DEBUG("CACHE: %sinclusive, %s, %s%u way(s), "
                          "%u set(s), line size %u, %u partition(s)\n",
                          (leaf4.edx & 2) ? "" : "not ",
                          (leaf4.edx & 4) ? "complex cache indexing" :
                          "direct mapped",
                          (leaf4.eax & 0x200) ? "fully associative, " : "",
                          ci.num_ways, ci.num_sets, ci.line_size,
                          ci.num_partitions);

                if (cache_level == 2)
                        m_l2 = ci;

                if (cache_level == 3)
                        m_l3 = ci;
        }

        if (!cache_level_shift[2] || !cache_level_shift[1])
                return -1; /* L1 or L2 not detected */

        apic->l2_shift = cache_level_shift[2];

        if (cache_level_shift[3])
                apic->l3_shift = cache_level_shift[3];
        else
                apic->l3_shift = apic->pkg_shift;
        return 0;
}

/**
 * @brief Discovers core and cache APICID information
 *
 * @param [out] apic structure to be filled in with details
 *
 * @return Operation status
 * @retval 0 OK
 * @retval -1 input parameter error
 * @retval -2 error detecting APIC masks for cores & packages
 * @retval -3 error detecting APIC masks for cache levels
 */
static int
detect_apic_masks(struct apic_info *apic)
{
        if (apic == NULL)
                return -1;

        memset(apic, 0, sizeof(*apic));

        if (detect_apic_core_masks(apic) != 0)
                return -2;

        if (detect_apic_cache_masks(apic) != 0)
                return -3;

        return 0;
}

/**
 * @brief Detects CPU information
 *
 * - schedules current task to run on \a cpu
 * - runs CPUID leaf 0xB to get cpu's APICID
 * - uses \a apic & APICID information to:
 *   - retrieve socket id (physical package id)
 *   - retrieve L3/LLC cluster id
 *   - retrieve L2/MLC cluster id
 *
 * @param [in] cpu logical cpu id used by OS
 * @param [in] apic information about APICID structure
 * @param [out] info CPU information structure to be filled by the function
 *
 * @return Operation status
 * @retval 0 OK
 * @retval -1 error
 */
static int
detect_cpu(const int cpu,
           const struct apic_info *apic,
           struct pqos_coreinfo *info)
{
        struct cpuid_out leafB;
        uint32_t apicid;

        if (set_affinity(cpu) != 0)
                return -1;

        lcpuid(0xb, 0, &leafB);
        apicid = leafB.edx;        /* x2APIC ID */

        info->lcore = cpu;
        info->socket = (apicid & apic->pkg_mask) >> apic->pkg_shift;
        info->l3_id = apicid >> apic->l3_shift;
        info->l2_id = apicid >> apic->l2_shift;

        LOG_DEBUG("Detected core %u, socket %u, "
                  "L2 ID %u, L3 ID %u, APICID %u\n",
                  info->lcore, info->socket,
                  info->l2_id, info->l3_id, apicid);

        return 0;
}

/**
 * @brief Builds CPU topology structure
 *
 * - saves current task CPU affinity
 * - retrieves number of processors in the system
 * - detects information on APICID structure
 * - for each processor in the system:
 *      - change affinity to run only on the processor
 *      - read APICID of current processor with CPUID
 *      - retrieve package & cluster data from the APICID
 * - restores initial task CPU affinity
 *
 * @return Pointer to CPU topology structure
 * @retval NULL on error
 */
static struct pqos_cpuinfo *
cpuinfo_build_topo(void)
{
        unsigned i, max_core_count, core_count = 0;
        struct pqos_cpuinfo *l_cpu = NULL;
        cpu_set_t current_mask;
        struct apic_info apic;

        if (get_affinity(&current_mask) != 0) {
                LOG_ERROR("Error retrieving CPU affinity mask!");
                return NULL;
        }

        max_core_count = sysconf(_SC_NPROCESSORS_CONF);
        if (max_core_count == 0) {
                LOG_ERROR("Zero processors in the system!");
                return NULL;
        }

        if (detect_apic_masks(&apic) != 0) {
                LOG_ERROR("Couldn't retrieve APICID structure information!");
                return NULL;
        }

        const size_t mem_sz = sizeof(*l_cpu) +
                (max_core_count * sizeof(struct pqos_coreinfo));

        l_cpu = (struct pqos_cpuinfo *)malloc(mem_sz);
        if (l_cpu == NULL) {
                LOG_ERROR("Couldn't allocate CPU topology structure!");
                return NULL;
        }
        l_cpu->mem_size = (unsigned) mem_sz;
        memset(l_cpu, 0, mem_sz);

        for (i = 0; i < max_core_count; i++)
                if (detect_cpu(i, &apic, &l_cpu->cores[core_count]) == 0)
                        core_count++;

        if (set_affinity_mask(&current_mask) != 0) {
                LOG_ERROR("Couldn't restore original CPU affinity mask!");
                free(l_cpu);
                return NULL;
        }

        l_cpu->l2 = m_l2;
        l_cpu->l3 = m_l3;
        l_cpu->num_cores = core_count;
        if (core_count == 0) {
                free(l_cpu);
                l_cpu = NULL;
        }

        return l_cpu;
}

/**
 * Detect number of logical processors on the machine
 * and their location.
 */
int
cpuinfo_init(const struct pqos_cpuinfo **topology)
{
        if (topology == NULL)
                return -EINVAL;

        if (m_cpu != NULL)
                return -EPERM;

        m_cpu = cpuinfo_build_topo();
        if (m_cpu == NULL) {
                LOG_ERROR("CPU topology detection error!");
                return -EFAULT;
        }

        *topology = m_cpu;
        return 0;
}

int
cpuinfo_fini(void)
{
        if (m_cpu == NULL)
                return -EPERM;
        free(m_cpu);
        m_cpu = NULL;
        return 0;
}
