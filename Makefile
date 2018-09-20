###############################################################################
# Makefile script for PQoS sample application
#
# @par
# BSD LICENSE
#
# Copyright(c) 2014-2016 Intel Corporation. All rights reserved.
# All rights reserved.
#
# Redistribution and use in source and binary forms, with or without
# modification, are permitted provided that the following conditions
# are met:
#
#   * Redistributions of source code must retain the above copyright
#     notice, this list of conditions and the following disclaimer.
#   * Redistributions in binary form must reproduce the above copyright
#     notice, this list of conditions and the following disclaimer in
#     the documentation and/or other materials provided with the
#     distribution.
#   * Neither the name of Intel Corporation nor the names of its
#     contributors may be used to endorse or promote products derived
#     from this software without specific prior written permission.
#
# THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
# "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
# LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
# A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
# OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
# SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
# LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
# DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
# THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
# (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
# OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
#
###############################################################################

# XXX: modify as desired
PREFIX ?= /usr/local
export PREFIX

ifdef DEBUG
export DEBUG
endif

ifdef SHARED
export SHARED
endif

.PHONY: all clean TAGS install uninstall style cppcheck

all:
	$(MAKE) -C lib
	$(MAKE) -C pqos
	$(MAKE) -C rdtset
	$(MAKE) -C examples/c/CAT
	$(MAKE) -C examples/c/CMT_MBM
	$(MAKE) -C examples/c/PSEUDO_LOCK

clean:
	$(MAKE) -C lib clean
	$(MAKE) -C pqos clean
	$(MAKE) -C rdtset clean
	$(MAKE) -C examples/c/CAT clean
	$(MAKE) -C examples/c/CMT_MBM clean
	$(MAKE) -C examples/c/PSEUDO_LOCK clean

style:
	$(MAKE) -C lib style
	$(MAKE) -C pqos style
	$(MAKE) -C rdtset style
	$(MAKE) -C examples/c/CAT style
	$(MAKE) -C examples/c/CMT_MBM style
	$(MAKE) -C examples/c/PSEUDO_LOCK style

cppcheck:
	$(MAKE) -C lib cppcheck
	$(MAKE) -C pqos cppcheck
	$(MAKE) -C rdtset cppcheck
	$(MAKE) -C examples/c/CAT cppcheck
	$(MAKE) -C examples/c/CMT_MBM cppcheck
	$(MAKE) -C examples/c/PSEUDO_LOCK cppcheck

install:
	$(MAKE) -C lib install
	$(MAKE) -C pqos install
	$(MAKE) -C rdtset install

uninstall:
	$(MAKE) -C lib uninstall
	$(MAKE) -C pqos uninstall
	$(MAKE) -C rdtset uninstall

TAGS:
	find ./ -name "*.[ch]" -print | etags -
