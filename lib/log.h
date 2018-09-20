/*
 * BSD LICENSE
 *
 * Copyright(c) 2014-2015 Intel Corporation. All rights reserved.
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
 * @brief Platform QoS operations logger for info, warnings and errors.
 */

#ifndef __PQOS_LOG_H__
#define __PQOS_LOG_H__

#include <stdio.h>
#include <stdarg.h>

#ifdef __cplusplus
extern "C" {
#endif

#define LOG_VER_SILENT          (-1)
#define LOG_VER_DEFAULT         (0)
#define LOG_VER_VERBOSE         (1)
#define LOG_VER_SUPER_VERBOSE   (2)

#define LOG_RETVAL_OK           0         /**< everything OK */
#define LOG_RETVAL_ERROR        1         /**< generic error */

#define LOG_OPT_INFO   (1 << 0)
#define LOG_OPT_WARN   (1 << 1)
#define LOG_OPT_ERROR  (1 << 2)
#define LOG_OPT_DEBUG  (1 << 3)

#define LOG_OPT_SILENT          (-1)
#define LOG_OPT_DEFAULT         (LOG_OPT_WARN|LOG_OPT_ERROR)
#define LOG_OPT_VERBOSE         (LOG_OPT_WARN|LOG_OPT_ERROR|LOG_OPT_INFO)
#define LOG_OPT_SUPER_VERBOSE   (LOG_OPT_WARN|LOG_OPT_ERROR|LOG_OPT_INFO| \
                                 LOG_OPT_DEBUG)

#define LOG_INFO(str...)  log_printf(LOG_OPT_INFO, "INFO: " str)
#define LOG_WARN(str...)  log_printf(LOG_OPT_WARN, "WARN: " str)
#define LOG_ERROR(str...) log_printf(LOG_OPT_ERROR, "ERROR: " str)
#define LOG_DEBUG(str...) log_printf(LOG_OPT_DEBUG, "DEBUG: " str)

/**
 * @brief Initializes PQoS log module
 * There are five typical use cases for this function
 *  [1] log to file descriptor only
 *  @note log_init(fd_log, NULL, NULL, LOG_VER_DEFAULT);
 *  [2] use callback function to capture logs
 *  @note log_init(-1, custom_callback, NULL, LOG_VER_DEFAULT);
 *  [3] use callback with a custom context
 *  @note log_init(-1, custom_callback, anything, LOG_VER_DEFAULT);
 *  [4] use both a callback and file descriptor
 *  @note log_init(fd_log, custom_callback, NULL, LOG_VER_DEFAULT);
 *  [5] keep all logging silent
 *  @note log_init(-1, NULL, NULL, LOG_VER_SILENT);
 *
 * @param [in] fd_log file descriptor to be used as library log
 * @param [in] callback_log pointer to an application callback function
 *         void *       - An application context - it can point to a structure
 *                        or an object that an application may find useful
 *                        when receiving the callback
 *         const size_t - the size of the log message
 *         const char * - the log message
 * @param [in] context_log application specific data that is provided
 *                    to the callback function. It can be NULL if application
 *                    doesn't require it.
 * @param [in] verbosity logging options
 *         LOG_VER_SILENT         - no messages
 *         LOG_VER_DEFAULT        - warning and error messages
 *         LOG_VER_VERBOSE        - warning, error and info messages
 *         LOG_VER_SUPER_VERBOSE  - warning, error, info and debug messages
 *
 * @return Operation status
 * @retval LOG_RETVAL_OK on success
 */
int log_init(int fd_log,
            void (*callback_log)(void *, const size_t, const char *),
            void *context_log,
            int verbosity);

/**
 * @brief Shuts down PQoS log module
 *
 * @return Operation status
 * @retval LOG_RETVAL_OK on success
 */
int log_fini(void);

/**
 * @brief PQoS log function
 *
 * @param [in] type log type to be made
 * @param [in] str format string compatible with printf().
 *             Variadic arguments to follow depending on \a str.
 */
void log_printf(int type, const char *str, ...);

#ifdef __cplusplus
}
#endif

#endif /* __PQOS_LOG_H__ */
