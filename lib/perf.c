/*
 * BSD LICENSE
 *
 * Copyright(c) 2015 Intel Corporation. All rights reserved.
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

#include <sys/syscall.h>
#include <sys/ioctl.h>

#include "types.h"
#include "pqos.h"
#include "perf.h"
#include "log.h"

/**
 * @brief Function to request a file descriptor to read perf counters
 *
 * @param attr perf event attribute structure
 * @param pid pid to monitor
 * @param cpu cpu to monitor
 * @param group_fd fd of group leader (-1 if no leader)
 * @param flags perf event flags
 *
 * @return fd used to read specified perf events
 * @retval positive number on success
 * @retval negative number on error
 */
static inline int
perf_event_open(struct perf_event_attr *attr, pid_t pid,
                int cpu, int group_fd, unsigned long flags)
{
        attr->size = sizeof(*attr);
        return syscall(__NR_perf_event_open, attr,
                       pid, cpu, group_fd, flags);
}

int
perf_setup_counter(struct perf_event_attr *attr,
                   const pid_t pid,
                   const int cpu,
                   const int group_fd,
                   const unsigned long flags,
                   int *counter_fd)
{
        int fd;

        if (attr == NULL || counter_fd == NULL)
                return PQOS_RETVAL_PARAM;

        fd = perf_event_open(attr, pid, cpu, group_fd, flags);
        if (fd < 0) {
                LOG_ERROR("Failed to open perf event!\n");
                return PQOS_RETVAL_ERROR;
        }
        *counter_fd = fd;

        return PQOS_RETVAL_OK;
}

int
perf_shutdown_counter(int counter_fd)
{
        int ret;

        if (counter_fd < 0)
                return PQOS_RETVAL_PARAM;

        ret = close(counter_fd);
        if (ret < 0) {
                LOG_ERROR("Failed to shutdown perf counter\n");
                return PQOS_RETVAL_ERROR;
        }

        return PQOS_RETVAL_OK;
}

int
perf_start_counter(int counter_fd)
{
        int ret;

        if (counter_fd <= 0)
                return PQOS_RETVAL_PARAM;

        ret = ioctl(counter_fd, PERF_EVENT_IOC_ENABLE);
        if (ret < 0) {
                LOG_ERROR("Failed to start perf counter!\n");
                return PQOS_RETVAL_ERROR;
        }

        return PQOS_RETVAL_OK;
}

int
perf_stop_counter(int counter_fd)
{
        int ret;

        if (counter_fd <= 0)
                return PQOS_RETVAL_PARAM;

        ret = ioctl(counter_fd, PERF_EVENT_IOC_DISABLE);
        if (ret < 0) {
                LOG_ERROR("Failed to stop perf counter!\n");
                return PQOS_RETVAL_ERROR;
        }

        return PQOS_RETVAL_OK;
}

int
perf_read_counter(int counter_fd, uint64_t *value)
{
        size_t res;

        if (counter_fd <= 0 || value == NULL)
                return PQOS_RETVAL_PARAM;

        res = read(counter_fd, value, sizeof(*value));
        if (res != sizeof(uint64_t)) {
                LOG_ERROR("Failed to read perf counter!\n");
                return PQOS_RETVAL_ERROR;
        }

        return  PQOS_RETVAL_OK;
}
