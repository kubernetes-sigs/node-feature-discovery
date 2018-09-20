#!/usr/bin/perl

###############################################################################
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

# PerlTidy options:
# -bar -ce -pt=2 -sbt=2 -bt=2 -bbt=2 -et=4 -baao -nsfs -vtc=1 -ci=4

use bigint;
use strict;
use warnings;

use pqos;

my $l2ca_cap_p;
my $l3ca_cap_p;
my $cap_p;
my $cpuinfo_p;
my $num_l2_cos;
my $num_l2_ways;
my $num_l3_cos;
my $num_l3_ways;
my $num_cores;
my $num_sockets;
my $sockets_a;
my $l2_ids_a;
my $num_l2_ids;

# Function to read the msr for a selected cpu
sub read_msr {
	my ($cpu_id, $msr) = @_;

	my $cmd;
	if ($^O eq "freebsd") {
		$cmd = sprintf("cpucontrol -m 0x%x /dev/cpuctl%u", $msr, $cpu_id);
	} else {
		$cmd = sprintf("rdmsr -p%u -0 -c 0x%x", $cpu_id, $msr);
	}

	my @result = `$cmd`;

	if (0 != $?) {
		print __LINE__, " $cmd FAILED!\n";
		return;
	}

	if ($^O eq "freebsd") {
		my @result_array = split / /, $result[0];
		return int(($result_array[2] << 32) + $result_array[3]);
	} else {
		return int(hex $result[0]);
	}
}

sub __func__ {
	return (caller(1))[3];
}

# Function to determine if msr tools are available
sub check_msr_tools {

	if ($^O eq "freebsd") {
		`cpucontrol -m 0xC8F /dev/cpuctl0`;
	} else {
		`rdmsr -p 0 -u 0xC8F`;
	}

	if (-1 == $?) {
		if ($^O eq "freebsd") {
			print __LINE__, " cpucontrol tool not found... ";
		} else {
			print __LINE__, " rdmsr tool not found... ";
		}
		print "please install.\n";
		return -1;
	}

	if (!defined read_msr(0, 0xC8F)) {
		print __LINE__, " unable to read MSR!...\n";
		return -1;
	}

	return 0;
}

# Function to shutdown libpqos
sub shutdown_pqos {
	print "Shutting down pqos lib...\n";
	pqos::pqos_fini();
	$l2ca_cap_p = undef;
	$l3ca_cap_p = undef;
	$cap_p      = undef;
	$cpuinfo_p  = undef;

	if (defined $sockets_a) {
		pqos::delete_uint_a($sockets_a);
		$sockets_a = undef;
	}

	if (defined $l2_ids_a) {
		pqos::delete_uint_a($l2_ids_a);
		$l2_ids_a = undef;
	}

	return;
}

# Function to initialize libpqos
sub init_pqos {
	my $cfg = pqos::pqos_config->new();
	my $ret = -1;

	$cfg->{verbose} = 2;                # SUPER_VERBOSE
	$cfg->{fd_log}  = fileno(STDOUT);

	if (0 != pqos::pqos_init($cfg)) {
		print __LINE__, " pqos::pqos_init FAILED!\n";
		goto EXIT;
	}

	$l2ca_cap_p = pqos::get_cap_l2ca();
	if (defined $l2ca_cap_p) {
		print __LINE__, " L2 CAT detected...\n";
		my $l2ca_cap = pqos::l2ca_cap_p_value($l2ca_cap_p);
		$num_l2_cos  = $l2ca_cap->{num_classes};
		$num_l2_ways = $l2ca_cap->{num_ways};
	}

	$l3ca_cap_p = pqos::get_cap_l3ca();
	if (defined $l3ca_cap_p) {
		print __LINE__, " L3 CAT detected...\n";
		my $l3ca_cap = pqos::l3ca_cap_p_value($l3ca_cap_p);
		$num_l3_cos  = $l3ca_cap->{num_classes};
		$num_l3_ways = $l3ca_cap->{num_ways};
	}

	if (!defined $l2ca_cap_p && !defined $l3ca_cap_p) {
		print __LINE__, " CAT not detected, ",
			"pqos::get_cap_l2ca && pqos::get_cap_l2ca FAILED!\n";
		goto EXIT;
	}

	my $cap_p_p     = pqos::new_pqos_cap_p_p();
	my $cpuinfo_p_p = pqos::new_pqos_cpuinfo_p_p();
	if (0 != pqos::pqos_cap_get($cap_p_p, $cpuinfo_p_p)) {
		print __LINE__, " pqos::pqos_cap_get FAILED!\n";
		goto EXIT;
	}
	$cap_p     = pqos::pqos_cap_p_p_value($cap_p_p);
	$cpuinfo_p = pqos::pqos_cpuinfo_p_p_value($cpuinfo_p_p);
	pqos::delete_pqos_cap_p_p($cap_p_p);
	pqos::delete_pqos_cpuinfo_p_p($cpuinfo_p_p);

	$num_cores = pqos::cpuinfo_p_value($cpuinfo_p)->{num_cores};

	($sockets_a, $num_sockets) = pqos::pqos_cpu_get_sockets($cpuinfo_p);
	if (0 == $sockets_a || 0 == $num_sockets) {
		print __LINE__, " pqos::pqos_cpu_get_sockets FAILED!\n";
		goto EXIT;
	}

	($l2_ids_a, $num_l2_ids) = pqos::pqos_cpu_get_l2ids($cpuinfo_p);
	if (0 == $l2_ids_a || 0 == $num_l2_ids) {
		print __LINE__, " pqos::pqos_cpu_get_l2ids FAILED!\n";
		goto EXIT;
	}
	$ret = 0;

EXIT:
	return sprintf("%s: %s", __func__, $ret == 0 ? "PASS" : "FAILED!");
}

# Function to generate CoS IDs - for testing purposes only
sub generate_cos {
	my ($num_cos, $cpu_id, $socket_id) = @_;

	return ($cpu_id + $socket_id) % $num_cos;
}

# Function to generate CoS's way mask - for testing purposes only
sub generate_ways_mask {
	my ($num_cos, $num_ways, $cos_id, $socket_id) = @_;

	my $bits_per_cos = int($num_ways / $num_cos);
	if ($bits_per_cos < 2) {
		$bits_per_cos = 2;
	}

	my $base_mask = (1 << ($bits_per_cos)) - 1;
	my $ways_mask =
		$base_mask
		<< (($cos_id * $bits_per_cos + $socket_id % 3) % ($num_ways - 1));
	my $result = $ways_mask & (2**$num_ways - 1);

	if (0 == $result) {
		$result = $base_mask;
	}

	return $result;
}

# Function to get CoS assigned to CPU via MSRs (using rdmsr from msr-tools)
sub get_msr_assoc {
	my ($cpu_id) = @_;
	return read_msr($cpu_id, 0xC8F) >> 32;
}

# Function to get L2 CoS ways mask via MSRs (using rdmsr from msr-tools)
sub get_msr_l2_ways_mask {
	return get_msr_ways_mask(@_, 0xD10);
}

# Function to get L3 CoS ways mask via MSRs (using rdmsr from msr-tools)
sub get_msr_l3_ways_mask {
	return get_msr_ways_mask(@_, 0xC90);
}

# Function to get CoS ways mask via MSRs (using rdmsr from msr-tools)
sub get_msr_ways_mask {
	my ($cos_id, $socket_id, $msr_mask_start) = @_;
	my $cpu_id;

	if (!defined $cpuinfo_p || !defined $num_cores) {
		print __LINE__, " !defined ... FAILED!\n";
		return;
	}

	for ($cpu_id = 0; $cpu_id < $num_cores; $cpu_id++) {
		(my $result, my $cpu_socket_id) =
			pqos::pqos_cpu_get_socketid($cpuinfo_p, $cpu_id);
		if (0 != $result) {
			print __LINE__, " pqos::pqos_cpu_get_socketid FAILED!\n";
			return;
		}

		if ($socket_id == $cpu_socket_id) {
			last;
		}
	}

	return read_msr($cpu_id, int($msr_mask_start + $cos_id)) & (2**32 - 1);
}

# Function to print current CAT configuration
sub print_cfg {
	my $cos_id;
	my $l2ca = pqos::pqos_l2ca->new();
	my $l3ca = pqos::pqos_l3ca->new();

	if (!defined $cpuinfo_p ||
		!defined $num_cores   ||
		!defined $num_sockets ||
		!defined $sockets_a   ||
		(!defined $num_l2_cos && !defined $num_l3_cos)) {
		return -1;
	}

	print "CoS configuration:\n";

	for (my $l2_idx = 0; $l2_idx < $num_l2_ids; $l2_idx++) {
		my $l2_id = pqos::uint_a_getitem($l2_ids_a, $l2_idx);

		for (
			$cos_id = 0;
			defined $num_l2_cos && $cos_id < $num_l2_cos;
			$cos_id++
			) {
			if (0 != pqos::get_l2ca($l2ca, $l2_id, $cos_id)) {
				print __LINE__, " pqos::get_l2ca FAILED!\n";
				return -1;
			}

			printf("L2, L2_ID: %d, CoS: %d, ways_mask: 0x%x\n",
				$l2_id, $l2ca->{class_id}, $l2ca->{ways_mask});
		}
	}

	for (my $socket_idx = 0; $socket_idx < $num_sockets; $socket_idx++) {
		my $socket_id = pqos::uint_a_getitem($sockets_a, $socket_idx);

		for (
			$cos_id = 0;
			defined $num_l3_cos && $cos_id < $num_l3_cos;
			$cos_id++
			) {
			if (0 != pqos::get_l3ca($l3ca, $socket_id, $cos_id)) {
				print __LINE__, " pqos::get_l3ca FAILED!\n";
				return -1;
			}

			printf("L3, Socket: %d, CoS: %d, CDP: %d",
				$socket_id, $l3ca->{class_id}, $l3ca->{cdp});
			if (int($l3ca->{cdp}) == 1) {
				printf(
					", data_mask: 0x%x, code_mask: 0x%x",
					$l3ca->{u}->{s}->{data_mask},
					$l3ca->{u}->{s}->{code_mask});
			} else {
				printf(", ways_mask: 0x%x", $l3ca->{u}->{ways_mask});
			}
			print "\n";
		}
	}

	print "CoS association:\n";

	for (my $cpu_id = 0; $cpu_id < $num_cores; $cpu_id++) {
		(my $result, $cos_id) = pqos::pqos_alloc_assoc_get($cpu_id);
		if (0 != $result) {
			print __LINE__, " pqos::pqos_alloc_assoc_get FAILED!\n";
			return -1;
		}

		($result, my $socket_id) =
			pqos::pqos_cpu_get_socketid($cpuinfo_p, $cpu_id);
		if (0 != $result) {
			print __LINE__, " pqos::pqos_cpu_get_socketid FAILED!\n";
			return -1;
		}

		print "Socket: ", $socket_id, ", Core: ", $cpu_id, ", CoS: ",
			$cos_id, "\n";
	}

	return 0;
}

# Function to test CoS association to CPUs
# Association is configured using libpqos API
# Association is verified using libpqos API and also by reading MSRs
sub test_assoc {
	my $cos_id;
	my $num_cos;
	my $gen_cos_id;
	my $socket_id;
	my $result;
	my $ret = -1;

	if (!defined $cpuinfo_p ||
		!defined $num_cores ||
		(!defined $num_l2_cos && !defined $num_l3_cos)) {
		print __LINE__, " No variables defined in test_assoc FAILED!\n";
		goto EXIT;
	}

	if (defined $num_l2_cos && defined $num_l3_cos) {
		$num_cos = $num_l2_cos <= $num_l3_cos ? $num_l3_cos : $num_l2_cos;
	} elsif (defined $num_l2_cos) {
		$num_cos = $num_l2_cos;
	} else {
		$num_cos = $num_l3_cos;
	}

	for (my $cpu_id = 0; $cpu_id < $num_cores; $cpu_id++) {
		($result, $cos_id) = pqos::pqos_alloc_assoc_get($cpu_id);
		if (0 != $result) {
			print __LINE__, " pqos::pqos_alloc_assoc_get FAILED!\n";
			goto EXIT;
		}

		($result, $socket_id) =
			pqos::pqos_cpu_get_socketid($cpuinfo_p, $cpu_id);
		if (0 != $result) {
			print __LINE__, " pqos::pqos_cpu_get_socketid FAILED!\n";
			goto EXIT;
		}

		$gen_cos_id = generate_cos($num_cos, $cpu_id, $socket_id);
		if (!defined $gen_cos_id) {
			print __LINE__, " generate_cos FAILED!\n";
			goto EXIT;
		}

		if (0 != pqos::pqos_alloc_assoc_set($cpu_id, $gen_cos_id)) {
			print __LINE__, " pqos::pqos_alloc_assoc_set FAILED!\n";
			goto EXIT;
		}
	}

	for (my $cpu_id = 0; $cpu_id < $num_cores; $cpu_id++) {
		($result, $cos_id) = pqos::pqos_alloc_assoc_get($cpu_id);
		if (0 != $result) {
			print __LINE__, " pqos::pqos_alloc_assoc_get FAILED!\n";
			goto EXIT;
		}

		($result, $socket_id) =
			pqos::pqos_cpu_get_socketid($cpuinfo_p, $cpu_id);
		if (0 != $result) {
			print __LINE__, " pqos::pqos_cpu_get_socketid FAILED!\n";
			goto EXIT;
		}

		$gen_cos_id = generate_cos($num_cos, $cpu_id, $socket_id);
		if (!defined $gen_cos_id) {
			print __LINE__, " generate_cos FAILED!\n";
			goto EXIT;
		}

		if ($cos_id != $gen_cos_id) {
			print __LINE__, ' $cos_id != $gen_cos_id FAILED!', "\n";
			goto EXIT;
		}

		$cos_id = get_msr_assoc($cpu_id);
		if (!defined $cos_id) {
			print __LINE__, " get_msr_assoc FAILED!\n";
			goto EXIT;
		}

		if ($cos_id != $gen_cos_id) {
			print __LINE__, " msr $cos_id != $gen_cos_id FAILED!\n";
			goto EXIT;
		}
	}

	$ret = 0;

EXIT:
	return sprintf("%s: %s", __func__, $ret == 0 ? "PASS" : "FAILED!");
}

# Function to test CoS ways masks configuration
# CoS is configured using libpqos API
# CoS configuration is verified using libpqos API and also by reading MSRs
sub test_way_masks {
	my $l2ca = pqos::pqos_l2ca->new();
	my $l3ca = pqos::pqos_l3ca->new();
	my $gen_ways_mask;
	my $msr_ways_mask;
	my $ret = -1;

	if (!defined $num_sockets ||
		!defined $sockets_a ||
		(!defined $num_l2_cos && !defined $num_l3_cos)) {
		print __LINE__, " No variables defined in test_way_masks, FAILED!\n";
		goto EXIT;
	}

	for (my $l2_idx = 0; $l2_idx < $num_l2_ids; $l2_idx++) {
		my $l2_id = pqos::uint_a_getitem($l2_ids_a, $l2_idx);
		if (!defined $l2_id) {
			print __LINE__, " pqos::uint_a_getitem FAILED!\n";
			goto EXIT;
		}

		for (
			my $cos_id = 0;
			defined $num_l2_cos && $cos_id < $num_l2_cos;
			$cos_id++
			) {

			$gen_ways_mask =
				generate_ways_mask($num_l2_cos, $num_l2_ways, $cos_id, $l2_id);

			if (!defined $gen_ways_mask) {
				print __LINE__, " L2 generate_ways_mask FAILED!\n";
				goto EXIT;
			}

			$l2ca->{ways_mask} = $gen_ways_mask;
			$l2ca->{class_id}  = $cos_id;

			if (0 != pqos::pqos_l2ca_set($l2_id, 1, $l2ca)) {
				print __LINE__, " pqos::pqos_l2ca_set FAILED!\n";
				goto EXIT;
			}
		}
	}

	for (my $socket_idx = 0; $socket_idx < $num_sockets; $socket_idx++) {
		my $socket_id = pqos::uint_a_getitem($sockets_a, $socket_idx);
		if (!defined $socket_id) {
			print __LINE__, " pqos::uint_a_getitem FAILED!\n";
			goto EXIT;
		}

		for (
			my $cos_id = 0;
			defined $num_l3_cos && $cos_id < $num_l3_cos;
			$cos_id++
			) {

			$gen_ways_mask = generate_ways_mask($num_l3_cos,
				$num_l3_ways, $cos_id, $socket_id);

			if (!defined $gen_ways_mask) {
				print __LINE__, " L3 generate_ways_mask FAILED!\n";
				goto EXIT;
			}

			$l3ca->{u}->{ways_mask} = $gen_ways_mask;
			$l3ca->{class_id}       = $cos_id;
			$l3ca->{cdp}            = 0;

			if (0 != pqos::pqos_l3ca_set($socket_id, 1, $l3ca)) {
				print __LINE__, " pqos::pqos_l3ca_set FAILED!\n";
				goto EXIT;
			}
		}
	}

	for (my $socket_idx = 0; $socket_idx < $num_sockets; $socket_idx++) {
		my $socket_id = pqos::uint_a_getitem($sockets_a, $socket_idx);
		if (!defined $socket_id) {
			print __LINE__, " pqos::uint_a_getitem FAILED!\n";
			goto EXIT;
		}

		for (
			my $cos_id = 0;
			defined $num_l2_cos && $cos_id < $num_l2_cos;
			$cos_id++
			) {

			if (0 != pqos::get_l2ca($l2ca, $socket_id, $cos_id)) {
				print __LINE__, " pqos::get_l2ca FAILED!\n";
				goto EXIT;
			}

			$gen_ways_mask = generate_ways_mask($num_l2_cos,
				$num_l2_ways, $cos_id, $socket_id);
			if (!defined $gen_ways_mask) {
				print __LINE__, " L2 generate_ways_mask FAILED!\n";
				goto EXIT;
			}

			if ($l2ca->{ways_mask} != $gen_ways_mask) {
				print __LINE__,
					' $l2ca->{ways_mask} != $gen_ways_mask ',
					"FAILED!\n";
				goto EXIT;
			}

			$msr_ways_mask = get_msr_l2_ways_mask($cos_id, $socket_id);
			if (!defined $msr_ways_mask) {
				print __LINE__, " get_msr_l2_ways_mask FAILED!\n";
				goto EXIT;
			}

			if ($msr_ways_mask != $gen_ways_mask) {
				print __LINE__,
					' L2 $msr_ways_mask != $gen_ways_mask ',
					"FAILED!\n";
				goto EXIT;
			}
		}

		for (
			my $cos_id = 0;
			defined $num_l3_cos && $cos_id < $num_l3_cos;
			$cos_id++
			) {

			if (0 != pqos::get_l3ca($l3ca, $socket_id, $cos_id)) {
				print __LINE__, " pqos::get_l3ca FAILED!\n";
				goto EXIT;
			}

			$gen_ways_mask = generate_ways_mask($num_l3_cos,
				$num_l3_ways, $cos_id, $socket_id);
			if (!defined $gen_ways_mask) {
				print __LINE__, " L3 generate_ways_mask FAILED!\n";
				goto EXIT;
			}

			if ($l3ca->{u}->{ways_mask} != $gen_ways_mask) {
				print __LINE__,
					' $l3ca->{u}->{ways_mask} != $gen_ways_mask',
					"FAILED!\n";
				goto EXIT;
			}

			$msr_ways_mask = get_msr_l3_ways_mask($cos_id, $socket_id);
			if (!defined $msr_ways_mask) {
				print __LINE__, " get_msr_l3_ways_mask FAILED!\n";
				goto EXIT;
			}

			if ($msr_ways_mask != $gen_ways_mask) {
				print __LINE__,
					' L3 $msr_ways_mask != $gen_ways_mask ',
					"FAILED!\n";
				goto EXIT;
			}
		}
	}

	$ret = 0;

EXIT:
	return sprintf("%s: %s", __func__, 0 == $ret ? "PASS" : "FAILED!");
}

# Function to test CMT LLC occupancy monitoring
# CMT is detected and LLC occupancy is polled using libpqos API
sub test_cmt_llc {
	my $ret              = -1;
	my $monitor_p_p      = pqos::new_pqos_monitor_p_p();
	my $llc_mon_data_p_a = pqos::new_pqos_mon_data_p_a($num_cores);
	my $cpu_id_p         = pqos::new_uintp();

	for (my $cpu_id = 0; $cpu_id < $num_cores; $cpu_id++) {
		pqos::pqos_mon_data_p_a_setitem($llc_mon_data_p_a, $cpu_id, undef);
	}

	if (
		0 == pqos::pqos_cap_get_event(
			$cap_p, $pqos::PQOS_MON_EVENT_L3_OCCUP, $monitor_p_p)
		) {
		my $llc_mon_data_p;

		for (my $cpu_id = 0; $cpu_id < $num_cores; $cpu_id++) {
			pqos::uintp_assign($cpu_id_p, $cpu_id);

			$llc_mon_data_p = pqos::new_pqos_mon_data_p();
			pqos::pqos_mon_data_p_a_setitem($llc_mon_data_p_a, $cpu_id,
				$llc_mon_data_p);

			if (
				0 != pqos::pqos_mon_start(
					1, $cpu_id_p, $pqos::PQOS_MON_EVENT_L3_OCCUP,
					undef, $llc_mon_data_p)
				) {
				print __LINE__, " pqos::pqos_mon_start FAILED!\n";
				goto EXIT;
			}
		}

		if (0 != pqos::pqos_mon_poll($llc_mon_data_p_a, $num_cores)) {
			print __LINE__, " pqos::pqos_mon_poll FAILED!\n";
			goto EXIT;
		}

		print "CMT LLC Occupancy:\n";

		for (my $cpu_id = 0; $cpu_id < $num_cores; $cpu_id++) {

			(my $result, my $socket_id) =
				pqos::pqos_cpu_get_socketid($cpuinfo_p, $cpu_id);
			if (0 != $result) {
				print __LINE__, " pqos::pqos_cpu_get_socketid FAILED!\n";
				goto EXIT;
			}

			$llc_mon_data_p =
				pqos::pqos_mon_data_p_a_getitem($llc_mon_data_p_a, $cpu_id);

			print "Socket: ", $socket_id, ", Core: ", $cpu_id,
				", LLC[KB]: ",
				pqos::pqos_mon_data_p_value($llc_mon_data_p)->{values}->{llc} /
				1024,
				"\n";

			if (0 != pqos::pqos_mon_stop($llc_mon_data_p)) {
				print __LINE__, " pqos::pqos_mon_stop FAILED!\n";
				goto EXIT;
			}
		}
	} else {
		print "CMT LLC monitoring capability not detected...\n";
	}

	$ret = 0;

EXIT:
	if (defined $cpu_id_p) {
		pqos::delete_uintp($cpu_id_p);
	}

	if (defined $monitor_p_p) {
		pqos::delete_pqos_monitor_p_p($monitor_p_p);
	}

	if (defined $llc_mon_data_p_a) {
		for (my $cpu_id = 0; $cpu_id < $num_cores; $cpu_id++) {
			my $llc_mon_data_p =
				pqos::pqos_mon_data_p_a_getitem($llc_mon_data_p_a, $cpu_id);
			if (defined $llc_mon_data_p) {
				pqos::delete_pqos_mon_data_p($llc_mon_data_p);
			}
		}

		pqos::delete_pqos_mon_data_p_a($llc_mon_data_p_a);
		$llc_mon_data_p_a = undef;
	}

	return sprintf("%s: %s", __func__, 0 == $ret ? "PASS" : "FAILED!");
}

# Function to reset CAT configuration - for testing purposes only
sub reset_cfg {
	my $l3ca = pqos::pqos_l3ca->new();
	my $l2ca = pqos::pqos_l2ca->new();
	my $cos_id;
	my $ret = -1;

	if ((!defined $num_l2_ways && !defined $num_l3_ways) ||
		!defined $num_cores   ||
		!defined $num_sockets ||
		!defined $sockets_a   ||
		(!defined $num_l2_cos && !defined $num_l3_cos)) {
		print __LINE__, " No variables defined in reset_cfg, FAILED!\n";
		goto EXIT;
	}

	for (my $cpu_id = 0; $cpu_id < $num_cores; $cpu_id++) {
		if (0 != pqos::pqos_alloc_assoc_set($cpu_id, 0)) {
			print __LINE__, " pqos::pqos_alloc_assoc_set FAILED!\n";
			goto EXIT;
		}
	}

	for (my $socket_idx = 0; $socket_idx < $num_sockets; $socket_idx++) {
		my $socket_id = pqos::uint_a_getitem($sockets_a, $socket_idx);
		for (
			$cos_id = 0;
			defined $num_l2_cos && $cos_id < $num_l2_cos;
			$cos_id++
			) {

			if (0 != pqos::get_l2ca($l2ca, $socket_id, $cos_id)) {
				print __LINE__, " pqos::get_l2ca FAILED!\n";
				goto EXIT;
			}

			$l2ca->{ways_mask} = (1 << $num_l2_ways) - 1;

			if (0 != pqos::pqos_l2ca_set($socket_id, 1, $l2ca)) {
				print __LINE__, " pqos::pqos_l2ca_set FAILED!\n";
				goto EXIT;
			}
		}

		for (
			$cos_id = 0;
			defined $num_l3_cos && $cos_id < $num_l3_cos;
			$cos_id++
			) {

			if (0 != pqos::get_l3ca($l3ca, $socket_id, $cos_id)) {
				print __LINE__, " pqos::get_l3ca FAILED!\n";
				goto EXIT;
			}

			$l3ca->{cdp} = 0;
			$l3ca->{u}->{ways_mask} = (1 << $num_l3_ways) - 1;

			if (0 != pqos::pqos_l3ca_set($socket_id, 1, $l3ca)) {
				print __LINE__, " pqos::pqos_l3ca_set FAILED!\n";
				goto EXIT;
			}
		}
	}

	$ret = 0;

EXIT:
	return sprintf("%s: %s", __func__, $ret == 0 ? "PASS" : "FAILED!");
}

(0 == check_msr_tools) or die("MSR reading tool issue...\n");

printf("%d %s\n", __LINE__, init_pqos());
printf("%d %s\n", __LINE__, test_assoc());
printf("%d %s\n", __LINE__, test_way_masks());

print_cfg;

printf("%d %s\n", __LINE__, reset_cfg());
printf("%d %s\n", __LINE__, test_cmt_llc());

shutdown_pqos();
