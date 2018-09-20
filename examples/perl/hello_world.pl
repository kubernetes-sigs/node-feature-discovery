#!/usr/bin/perl

################################################################################
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
################################################################################

use strict;
use warnings;
use pqos;

=begin
Script to test PQoS Perl interface
=cut

my $cfg  = pqos::pqos_config->new();
my $l3ca = pqos::pqos_l3ca->new();

# Setup config
$cfg->{verbose} = 0;
$cfg->{fd_log}  = 1;

# Initialize the library
if (0 != pqos::pqos_init($cfg)) {
	print "Error initializing PQoS library!\n";
	exit 0;
}
print "Hello, World!\n";
print "\t\t\t\tAssociation\tWay Mask\n";

# Get number of cores
my $cpuinfo_p = pqos::get_cpuinfo();
my $cpu_num   = pqos::cpuinfo_p_value($cpuinfo_p)->{num_cores};

# Print L3CA info for each core
for (my $i = 0; $i < $cpu_num; $i++) {

	# Get core association
	(my $result, my $cos) = pqos::pqos_alloc_assoc_get($i);
	if (0 != $result) {
		next;
	}

	# Get socket ID for this core
	($result, my $socket_id) = pqos::pqos_cpu_get_socketid($cpuinfo_p, $i);
	if (0 != $result) {
		next;
	}

	# Get way mask info
	if (0 != pqos::get_l3ca($l3ca, $socket_id, $cos)) {
		next;
	}

	# Print info
	printf("Hello from core %d on socket %d\tCOS %d \t\t0x%x\n",
		$i, $socket_id, $cos, $l3ca->{u}->{ways_mask});
}

# Shutdown the library
pqos::pqos_fini();
