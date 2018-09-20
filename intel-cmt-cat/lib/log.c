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
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.O
 *
 */

/**
 * @brief Library operations logger for info, warnings and errors.
 */

#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>
#include <string.h>
#include <unistd.h>
#ifdef __linux__
#include <error.h>
#endif /* __linux__ */
#include <errno.h>

#include "types.h"
#include "log.h"

/**
 * ---------------------------------------
 * Local data types
 * ---------------------------------------
 */
#define AP_BUFFER_SIZE  256

/**
 * ---------------------------------------
 * Local data structures
 * ---------------------------------------
 */

static int m_opt = 0;                   /**< log options */
static int m_fd = -1;			/**< log file descriptor */
static void *m_context_log = NULL;      /**< log callback context */
/**
 *  log callback
 */
static void (*m_callback_log)(void *, const size_t, const char *);
static int log_init_successful = 0;     /**< log init gatekeeper */
/**
 * ---------------------------------------
 * Local functions
 * ---------------------------------------
 */


/**
 * =======================================
 * initialize and shutdown
 * =======================================
 */

int
log_init(int fd_log, void (*callback_log)(void *, const size_t, const char *),
        void *context_log, int verbosity)
{
	/**
         * Set log message verbosity
         */
        switch (verbosity) {
        case LOG_VER_SILENT:
                m_opt = LOG_OPT_SILENT;
                log_init_successful = 1;
		return LOG_RETVAL_OK;
        case LOG_VER_DEFAULT:
                m_opt = LOG_OPT_DEFAULT;
                break;
        case LOG_VER_VERBOSE:
                m_opt = LOG_OPT_VERBOSE;
                break;
        case LOG_VER_SUPER_VERBOSE:
                m_opt = LOG_OPT_SUPER_VERBOSE;
                break;
        default:
                m_opt = LOG_OPT_SUPER_VERBOSE;
                break;
        }

	if (fd_log < 0 && callback_log == NULL) {
                fprintf(stderr, "%s: no LOG destination selected\n",
                       __func__);
                return LOG_RETVAL_ERROR;
        }

	m_fd = fd_log;
	m_callback_log = callback_log;
	m_context_log = context_log;
	log_init_successful = 1;

        return LOG_RETVAL_OK;
}

int
log_fini(void)
{
	if (m_opt == LOG_OPT_SILENT) {
                log_init_successful = 0;
		return LOG_RETVAL_OK;
        }

        m_opt = 0;
	m_fd = -1;
	m_callback_log = NULL;
	m_context_log = NULL;
	log_init_successful = 0;

        return LOG_RETVAL_OK;
}

void
log_printf(int type, const char *str, ...)
{
        va_list ap;
	char ap_buffer[AP_BUFFER_SIZE];
        int size;

	/* If log_init has not been successful then
	 * log_printf should not work. */
        ASSERT(log_init_successful == 1);
	if (log_init_successful == 0)
		return;

	if (m_opt == LOG_OPT_SILENT)
		return;

        if ((m_opt & type) == 0)
                return;

        ASSERT(str != NULL);
        if (str == NULL)
                return;

	va_start(ap, str);
	ap_buffer[AP_BUFFER_SIZE - 1] = '\0';
	size = vsnprintf(ap_buffer, AP_BUFFER_SIZE - 1, str, ap);
	va_end(ap);
	ASSERT(size >= 0);
	if (size < 0)
		return;

	if (m_callback_log != NULL)
		m_callback_log(m_context_log, size, ap_buffer);

	if (m_fd >= 0) {
		if (write(m_fd, ap_buffer, size) < 0)
			fprintf(stderr, "%s: printing to file failed\n",
                                __func__);
	}
}
