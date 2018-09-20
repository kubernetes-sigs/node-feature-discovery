###############################################################################
# Makefile script for RDT SNMP Subagent Perl script
#
# @par
# BSD LICENSE
#
# Copyright(c) 2016 Intel Corporation. All rights reserved.
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
###############################################################################

PERLTIDY?=perltidy

PERLTIDY_STYLE=-bar -ce -pt=2 -sbt=2 -bt=2 -bbt=2 -et=4 -baao -nsfs -vtc=1 -ci=4
# http://perltidy.sourceforge.net/stylekey.html
# http://perltidy.sourceforge.net/perltidy.html
# -bar - keep the brace on the right even for multiple-line test expressions
# -ce - cuddled else
# -pt=2 - parentheses horizontal tightness - space is never used
# -sbt=2 - square brackets horizontal tightness - space is never used
# -bbt=2 - code block curly braces horizontal tightness - space is never used
# -et=4 - entab leading whitespace with one tab character for each 4 spaces
# -baao - "break after all operators"
# -nsfs - no space before a semicolon in a for statement
# -vtc=1 - do not break before a closing token which is followed by a semicolon
#          or another closing token, and is not in a list environment
# -ci=4 - continuation indentation - extra indentation spaces applied when
#         a long line is broken, 4 spaces

# List of Perl scripts to be processed, space separated list
PERL_SCRIPTS_LIST=rdt-agentx.pl

# To check if required style is followed, perltidy is called and
# formatted output is compared to actual file,
# if there are no differences, it means that file meets the requirements
.PHONY: style
style:
	@bash -c 'for i in $(PERL_SCRIPTS_LIST); do \
		$(PERLTIDY) $(PERLTIDY_STYLE) ./$${i} -st |\
		diff - ./$${i} && echo "$${i} style OK" ||\
		{ echo "$${i} style check failed!"; exit 1; }; \
	done'

.PHONY: style-fix
style-fix:
	@bash -c 'for i in $(PERL_SCRIPTS_LIST); do \
		$(PERLTIDY) $(PERLTIDY_STYLE) -b ./$${i}; \
	done'
