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
 * @brief Set of utility functions to operate on Platform QoS (pqos) data
 *        structures.
 *
 * These functions need no synchronization mechanisms.
 *
 */
#include <stdlib.h>
#include <string.h>
#ifdef __linux__
#include <alloca.h>       /* alloca() */
#endif /* __linux__ */

#include "pqos.h"
#include "types.h"
#include "utils.h"

#define TOPO_OBJ_SOCKET     0
#define TOPO_OBJ_L2_CLUSTER 2
#define TOPO_OBJ_L3_CLUSTER 3

static int m_interface = PQOS_INTER_MSR;


int
_pqos_utils_init(int interface)
{
    m_interface = interface;

    return PQOS_RETVAL_OK;
}


unsigned *
pqos_cpu_get_sockets(const struct pqos_cpuinfo *cpu,
                     unsigned *count)
{
        unsigned scount = 0, i = 0;
        unsigned *sockets = NULL;

        ASSERT(cpu != NULL);
        ASSERT(count != NULL);
        if (cpu == NULL || count == NULL)
                return NULL;

        sockets = (unsigned *) malloc(sizeof(sockets[0]) * cpu->num_cores);
        if (sockets == NULL)
                return NULL;

        for (i = 0; i < cpu->num_cores; i++) {
                unsigned j = 0;

                /**
                 * Check if this socket id is already on the \a sockets list
                 */
                for (j = 0; j < scount && scount > 0; j++)
                        if (cpu->cores[i].socket == sockets[j])
                                break;

                if (j >= scount || scount == 0) {
                        /**
                         * This socket wasn't reported before
                         */
                        sockets[scount++] = cpu->cores[i].socket;
                }
        }

        *count = scount;
        return sockets;
}

unsigned *
pqos_cpu_get_l2ids(const struct pqos_cpuinfo *cpu,
                   unsigned *count)
{
        unsigned l2count = 0, i = 0;
        unsigned *l2ids = NULL;

        ASSERT(cpu != NULL);
        ASSERT(count != NULL);
        if (cpu == NULL || count == NULL)
                return NULL;

        l2ids = (unsigned *) malloc(sizeof(l2ids[0]) * cpu->num_cores);
        if (l2ids == NULL)
                return NULL;

        for (i = 0; i < cpu->num_cores; i++) {
                unsigned j = 0;

                /**
                 * Check if this L2 id is already on the list
                 */
                for (j = 0; j < l2count && l2count > 0; j++)
                        if (cpu->cores[i].l2_id == l2ids[j])
                                break;

                if (j >= l2count || l2count == 0) {
                        /**
                         * This l2id wasn't reported before
                         */
                        l2ids[l2count++] = cpu->cores[i].l2_id;
                }
        }

        *count = l2count;
        return l2ids;
}

/**
 * @brief Creates list of cores belonging to given topology object
 *
 * @param [in] cpu CPU topology
 * @param [in] type CPU topology object type to search cores for
 *             TOPO_OBJ_SOCKET - sockets
 *             TOPO_OBJ_L2_CLUSTER - L2 cache clusters
 *             TOPO_OBJ_L3_CLUSTER - L3 cache clusters
 * @param [in] id CPU topology object ID to search cores for
 * @param [out] count place to put number of objects found
 *
 * @return Pointer to list of cores for given topology object
 * @retval NULL on error or if no core found
 */
static unsigned *
__get_cores_per_topology_obj(const struct pqos_cpuinfo *cpu,
                             const int type,
                             const unsigned id,
                             unsigned *count)
{
        unsigned num = 0, i = 0;
        unsigned *core_list = NULL;

        ASSERT(cpu != NULL);
        ASSERT(count != NULL);
        if (cpu == NULL || count == NULL)
                return NULL;

        core_list = (unsigned *) malloc(cpu->num_cores * sizeof(core_list[0]));
        if (core_list == NULL)
                return NULL;

        for (i = 0; i < cpu->num_cores; i++)
                if ((type == TOPO_OBJ_L3_CLUSTER &&
                     id == cpu->cores[i].l3_id) ||
                    (type == TOPO_OBJ_L2_CLUSTER &&
                     id == cpu->cores[i].l2_id) ||
                    (type == TOPO_OBJ_SOCKET && id == cpu->cores[i].socket))
                        core_list[num++] = cpu->cores[i].lcore;

        if (num == 0) {
                free(core_list);
                return NULL;
        }

        *count = num;
        return core_list;
}

unsigned *
pqos_cpu_get_cores_l3id(const struct pqos_cpuinfo *cpu, const unsigned l3_id,
                        unsigned *count)
{
        return __get_cores_per_topology_obj(cpu, TOPO_OBJ_L3_CLUSTER, l3_id,
                                            count);
}

unsigned *
pqos_cpu_get_cores(const struct pqos_cpuinfo *cpu,
                   const unsigned socket,
                   unsigned *count)
{
        unsigned i = 0, cnt = 0;
        unsigned *cores = NULL;

        ASSERT(cpu != NULL);
        ASSERT(count != NULL);

        if (cpu == NULL || count == NULL)
                return NULL;

        cores = (unsigned *) malloc(cpu->num_cores * sizeof(cores[0]));
        if (cores == NULL)
                return NULL;

        for (i = 0; i < cpu->num_cores; i++)
                if (cpu->cores[i].socket == socket)
                        cores[cnt++] = cpu->cores[i].lcore;

        if (!cnt) {
                free(cores);
                return NULL;
        }

        *count = cnt;
        return cores;
}

const struct pqos_coreinfo *
pqos_cpu_get_core_info(const struct pqos_cpuinfo *cpu, unsigned lcore)
{
        unsigned i;

        ASSERT(cpu != NULL);

        if (cpu == NULL)
                return NULL;

        for (i = 0; i < cpu->num_cores; i++)
                if (cpu->cores[i].lcore == lcore)
                        return &cpu->cores[i];

        return NULL;
}

int
pqos_cpu_get_one_core(const struct pqos_cpuinfo *cpu,
                      const unsigned socket,
                      unsigned *lcore)
{
        unsigned i = 0;

        ASSERT(cpu != NULL);
        ASSERT(lcore != NULL);

        if (cpu == NULL || lcore == NULL)
                return PQOS_RETVAL_PARAM;

        for (i = 0; i < cpu->num_cores; i++)
                if (cpu->cores[i].socket == socket) {
                        *lcore = cpu->cores[i].lcore;
                        return PQOS_RETVAL_OK;
                }

        return PQOS_RETVAL_ERROR;
}

int
pqos_cpu_get_one_by_l2id(const struct pqos_cpuinfo *cpu,
                         const unsigned l2id,
                         unsigned *lcore)
{
        unsigned i = 0;

        ASSERT(cpu != NULL);
        ASSERT(lcore != NULL);

        if (cpu == NULL || lcore == NULL)
                return PQOS_RETVAL_PARAM;

        for (i = 0; i < cpu->num_cores; i++)
                if (cpu->cores[i].l2_id == l2id) {
                        *lcore = cpu->cores[i].lcore;
                        return PQOS_RETVAL_OK;
                }

        return PQOS_RETVAL_ERROR;
}

int
pqos_cpu_check_core(const struct pqos_cpuinfo *cpu,
                    const unsigned lcore)
{
        unsigned i = 0;

        ASSERT(cpu != NULL);
        if (cpu == NULL)
                return PQOS_RETVAL_PARAM;

        for (i = 0; i < cpu->num_cores; i++)
                if (cpu->cores[i].lcore == lcore)
                        return PQOS_RETVAL_OK;

        return PQOS_RETVAL_ERROR;
}

int
pqos_cpu_get_socketid(const struct pqos_cpuinfo *cpu,
                      const unsigned lcore,
                      unsigned *socket)
{
        unsigned i = 0;

        if (cpu == NULL || socket == NULL)
                return PQOS_RETVAL_PARAM;

        for (i = 0; i < cpu->num_cores; i++)
                if (cpu->cores[i].lcore == lcore) {
                        *socket = cpu->cores[i].socket;
                        return PQOS_RETVAL_OK;
                }

        return PQOS_RETVAL_ERROR;
}

int
pqos_cpu_get_clusterid(const struct pqos_cpuinfo *cpu,
                       const unsigned lcore,
                       unsigned *cluster)
{
        unsigned i = 0;

        if (cpu == NULL || cluster == NULL)
                return PQOS_RETVAL_PARAM;

        for (i = 0; i < cpu->num_cores; i++)
                if (cpu->cores[i].lcore == lcore) {
                        *cluster = cpu->cores[i].l3_id;
                        return PQOS_RETVAL_OK;
                }

        return PQOS_RETVAL_ERROR;
}

int
pqos_cap_get_type(const struct pqos_cap *cap,
                  const enum pqos_cap_type type,
                  const struct pqos_capability **cap_item)
{
        int ret = PQOS_RETVAL_RESOURCE;
        unsigned i;

        ASSERT(cap != NULL && cap_item != NULL);
        if (cap == NULL || cap_item == NULL)
                return PQOS_RETVAL_PARAM;

        ASSERT(type < PQOS_CAP_TYPE_NUMOF);
        if (type >= PQOS_CAP_TYPE_NUMOF)
                return PQOS_RETVAL_PARAM;

        for (i = 0; i < cap->num_cap; i++) {
                if (cap->capabilities[i].type != type)
                        continue;
                /**
                 * Feature not supported for OS interface
                 */
                if (m_interface == PQOS_INTER_OS &&
                    !cap->capabilities[i].os_support)
                        continue;
                *cap_item = &cap->capabilities[i];
                ret = PQOS_RETVAL_OK;
                break;
        }

        return ret;
}

int
pqos_cap_get_event(const struct pqos_cap *cap,
                   const enum pqos_mon_event event,
                   const struct pqos_monitor **p_mon)
{
        const struct pqos_capability *cap_item = NULL;
        const struct pqos_cap_mon *mon = NULL;
        int ret = PQOS_RETVAL_OK;
        unsigned i;

        if (cap == NULL || p_mon == NULL)
                return PQOS_RETVAL_PARAM;

        ret = pqos_cap_get_type(cap, PQOS_CAP_TYPE_MON, &cap_item);
        if (ret != PQOS_RETVAL_OK)
                return ret;

        ASSERT(cap_item != NULL);
        mon = cap_item->u.mon;

        ret = PQOS_RETVAL_ERROR;

        for (i = 0; i < mon->num_events; i++) {
                if (mon->events[i].type != event)
                        continue;
                /**
                 * Feature not supported for OS interface
                 */
                if (m_interface == PQOS_INTER_OS && !mon->events[i].os_support)
                        continue;

                *p_mon = &mon->events[i];
                ret = PQOS_RETVAL_OK;
                break;
        }

        return ret;
}

int
pqos_l3ca_get_cos_num(const struct pqos_cap *cap,
                      unsigned *cos_num)
{
        const struct pqos_capability *item = NULL;
        int ret = PQOS_RETVAL_OK;

        ASSERT(cap != NULL && cos_num != NULL);
        if (cap == NULL || cos_num == NULL)
                return PQOS_RETVAL_PARAM;

        ret = pqos_cap_get_type(cap, PQOS_CAP_TYPE_L3CA, &item);
        if (ret != PQOS_RETVAL_OK)
                return ret;                           /**< no L3CA capability */

        ASSERT(item != NULL);
        *cos_num = item->u.l3ca->num_classes;
        return ret;
}

int
pqos_l2ca_get_cos_num(const struct pqos_cap *cap,
                      unsigned *cos_num)
{
        const struct pqos_capability *item = NULL;
        int ret = PQOS_RETVAL_OK;

        ASSERT(cap != NULL && cos_num != NULL);
        if (cap == NULL || cos_num == NULL)
                return PQOS_RETVAL_PARAM;

        ret = pqos_cap_get_type(cap, PQOS_CAP_TYPE_L2CA, &item);
        if (ret != PQOS_RETVAL_OK)
                return ret;                           /**< no L2CA capability */

        ASSERT(item != NULL);
        *cos_num = item->u.l2ca->num_classes;
        return ret;
}

int
pqos_mba_get_cos_num(const struct pqos_cap *cap,
                      unsigned *cos_num)
{
        const struct pqos_capability *item = NULL;
        int ret = PQOS_RETVAL_OK;

        ASSERT(cap != NULL && cos_num != NULL);
        if (cap == NULL || cos_num == NULL)
                return PQOS_RETVAL_PARAM;

        ret = pqos_cap_get_type(cap, PQOS_CAP_TYPE_MBA, &item);
        if (ret != PQOS_RETVAL_OK)
                return ret;                           /**< no MBA capability */

        ASSERT(item != NULL);
        *cos_num = item->u.mba->num_classes;
        return ret;
}

int
pqos_l3ca_cdp_enabled(const struct pqos_cap *cap,
                      int *cdp_supported,
                      int *cdp_enabled)
{
        const struct pqos_capability *item = NULL;
        int ret = PQOS_RETVAL_OK;

        ASSERT(cap != NULL && (cdp_enabled != NULL || cdp_supported != NULL));
        if (cap == NULL || (cdp_enabled == NULL && cdp_supported == NULL))
                return PQOS_RETVAL_PARAM;

        ret = pqos_cap_get_type(cap, PQOS_CAP_TYPE_L3CA, &item);
        if (ret != PQOS_RETVAL_OK)
                return ret;                           /**< no L3CA capability */

        ASSERT(item != NULL);
        if (cdp_supported != NULL)
                *cdp_supported = item->u.l3ca->cdp;
        if (cdp_enabled != NULL)
                *cdp_enabled = item->u.l3ca->cdp_on;
        return ret;
}
