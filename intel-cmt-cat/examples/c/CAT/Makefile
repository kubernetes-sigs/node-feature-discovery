###############################################################################
# Makefile script for CAT PQoS sample applications
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

LIBDIR ?= ../../../lib
CFLAGS =-I$(LIBDIR) \
	-W -Wall -Wextra -Wstrict-prototypes -Wmissing-prototypes \
	-Wmissing-declarations -Wold-style-definition -Wpointer-arith \
	-Wcast-qual -Wundef -Wwrite-strings  \
        -Wformat -Wformat-security -fstack-protector -fPIE -D_FORTIFY_SOURCE=2 \
        -Wunreachable-code -Wmissing-noreturn -Wsign-compare -Wno-endif-labels \
	-g -O2
ifneq ($(EXTRA_CFLAGS),)
CFLAGS += $(EXTRA_CFLAGS)
endif
LDFLAGS=-L$(LIBDIR)
LDLIBS=-lpqos -lpthread

# ICC and GCC options
ifeq ($(CC),icc)
else
CFLAGS += -Wcast-align -Wnested-externs
endif

# Build targets and dependencies
ALLOCAPP = allocation_app
ASSOCAPP = association_app
RESETAPP = reset_app

all: $(ALLOCAPP) $(ASSOCAPP) $(RESETAPP)

$(ALLOCAPP): $(ALLOCAPP).o
$(ASSOCAPP): $(ASSOCAPP).o
$(RESETAPP): $(RESETAPP).o

.PHONY: clean
clean:
	-rm -f $(ALLOCAPP) $(ASSOCAPP) $(RESETAPP) *.o

CHECKPATCH?=checkpatch.pl
.PHONY: style
style:
	$(CHECKPATCH) --no-tree --no-signoff --emacs \
	--ignore CODE_INDENT,INITIALISED_STATIC,LEADING_SPACE,SPLIT_STRING,UNSPECIFIED_INT \
	-f allocation_app.c -f association_app.c -f reset_app.c

CPPCHECK?=cppcheck
.PHONY: cppcheck
cppcheck:
	$(CPPCHECK) --enable=warning,portability,performance,unusedFunction,missingInclude \
	--std=c99 -I$(LIBDIR) --template=gcc \
	allocation_app.c association_app.c reset_app.c
