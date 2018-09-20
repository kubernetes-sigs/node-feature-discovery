#!/usr/bin/perl
###############################################################################
#
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

=head1 NAME

    rdt-agentx.pl - Net-SNMP AgentX subagent

=head1 DESCRIPTION

    This is a Net-SNMP AgentX subagent written in Perl to demonstrate
    the use of the PQoS library Perl wrapper API. This subagent allows to
    read and change CAT configuration and to monitor LLC occupancy using
    CMT via SNMP.

    Supported OIDs:
    COS configuration OID: SNMPv2-SMI::enterprises.343.3.0.x.y.z
    x - socket id
    y - cos id
    z - field (cdp (RO): 1, ways_mask: 2, data_mask: 3, code_mask: 4)

    value:
    - z = 1, current CDP setting, Read-Only
    - z = 2, ways_mask configuration (if CDP == 0)
    - z = 3, data_mask configuration (if CDP == 1)
    - z = 4, code_mask configuration (if CDP == 1)

    Core properties CAT/Monitoring OID: SNMPv2-SMI::enterprises.343.3.1.x.y.z
    x - socket id
    y - cpu id
    z - field (COS ID: 0, LLC occupancy(RO): 1)

    value:
    - z = 0, CAT, COS ID assigned
    - z = 1, CMT, LLC occupancy, bytes, Read-Only

    Monitoring Control OID: SNMPv2-SMI::enterprises.343.3.2.x.y
    x - monitoring technology (CMT: 0)
    y - field (CMT supported (RO): 0, CMT state: 1)

    value:
    - y = 0, CMT, supported, true(1)/false(0), Read-Only
    - y = 1, CMT, current state, enable(1)/disable(0)

=cut

use bigint;
use strict;
use warnings;
use Readonly;

use NetSNMP::agent (':all');
use NetSNMP::ASN   (':all');
use NetSNMP::OID;

use pqos;

my %data_cos            = ();
my @data_cos_keys       = ();
my %data_core_prop      = ();
my @data_core_prop_keys = ();
my %data_mon_ctrl       = ();
my @data_mon_ctrl_keys  = ();
my $data_timestamp      = 0;

Readonly my $DATA_TIMEOUT => 1;

Readonly::Scalar my $OID_NUM_ENTER        => ".1.3.6.1.4.1";
Readonly::Scalar my $OID_NUM_INTEL        => "$OID_NUM_ENTER.343";
Readonly::Scalar my $OID_NUM_EXPERIMENTAL => "$OID_NUM_INTEL.3";

Readonly::Scalar my $OID_NUM_PQOS_COS => "$OID_NUM_EXPERIMENTAL.0";
Readonly my $COS_CDP_ID               => 0;
Readonly my $COS_WAYS_MASK_ID         => 1;
Readonly my $COS_DATA_MASK_ID         => 2;
Readonly my $COS_CODE_MASK_ID         => 3;

Readonly::Scalar my $OID_NUM_PQOS_CORE_PROP => "$OID_NUM_EXPERIMENTAL.1";
Readonly my $CORE_COS_ID                    => 0;
Readonly my $CORE_CMT_LLC_ID                => 1;

Readonly::Scalar my $OID_NUM_PQOS_MON_CTRL => "$OID_NUM_EXPERIMENTAL.2";
Readonly my $MON_CMT_ID                    => 0;
Readonly my $MON_LLC_SUPP_ID               => 0;
Readonly my $MON_LLC_STATE_ID              => 1;

my $oid_pqos_cos;
my $oid_pqos_core_prop;
my $oid_pqos_mon_ctrl;

my $l3ca_cap_p;
my $cap_p;
my $cpuinfo_p;

my $num_cos;
my $num_ways;
my $num_cores;
my $num_sockets;
my $sockets_a;

my $active = 1;

my $llc_mon_supp;
my $llc_mon_data_p_a;

=item shutdown_agent()

Shutdowns AgentX agent

=cut

sub shutdown_agent {
	if (defined $sockets_a) {
		pqos::delete_uint_a($sockets_a);
		$sockets_a = undef;
	}

	if (defined $llc_mon_data_p_a) {
		cmt_llc_stop();
	}

	print "Shutting down pqos lib...\n";
	pqos::pqos_fini();
	$active = 0;
	return;
}

=item pqos_init()

 Returns: 0 on success, -1 otherwise

Initializes PQoS library

=cut

sub pqos_init {
	my $cfg = pqos::pqos_config->new();

	$cfg->{verbose} = 2;                # SUPER_VERBOSE
	$cfg->{fd_log}  = fileno(STDOUT);

	my $interface_id = $ARGV[0];

        if ($interface_id eq '-I' || $interface_id eq '--iface-os') {
                $cfg->{interface} = 1;
        } else {
                $cfg->{interface} = 0;
        }

        if (0 != pqos::pqos_init($cfg)) {
		print __LINE__, " pqos::pqos_init FAILED!\n";
		return -1;
	}

	$l3ca_cap_p = pqos::get_cap_l3ca();
	if (defined $l3ca_cap_p) {
		print __LINE__, " L3CA capability detected...\n";
		my $l3ca_cap = pqos::l3ca_cap_p_value($l3ca_cap_p);
		$num_cos  = $l3ca_cap->{num_classes};
		$num_ways = $l3ca_cap->{num_ways};
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

	my $monitor_p_p = pqos::new_pqos_monitor_p_p();
	if (
		0 == pqos::pqos_cap_get_event(
			$cap_p, $pqos::PQOS_MON_EVENT_L3_OCCUP, $monitor_p_p)
		) {
		print __LINE__, " LLC monitoring capability detected...\n";
		$llc_mon_supp = 1;
	}
	pqos::delete_pqos_monitor_p_p($monitor_p_p);

	if (!defined $l3ca_cap_p && !defined $llc_mon_supp) {
		print __LINE__, " No L3CA or LLC monitoring detected... FAILED!\n";
		return -1;
	}

	$num_cores = pqos::cpuinfo_p_value($cpuinfo_p)->{num_cores};

	($sockets_a, $num_sockets) = pqos::pqos_cpu_get_sockets($cpuinfo_p);
	if (0 == $sockets_a || 0 == $num_sockets) {
		print __LINE__, " pqos::pqos_cpu_get_sockets FAILED!\n";
		return -1;
	}

	return 0;
}

=item data_update()

Reads current CAT configuration and CMT state and values
and updates local hashes with OIDs and values

=cut

sub data_update {
	my $now = time;
	if (($now - $data_timestamp) < $DATA_TIMEOUT) {
		return;
	}
	$data_timestamp = $now;

	data_update_core_prop();

	data_update_cos();

	data_update_mon_ctrl();

	return;
}

=item data_update_core_prop()

Handles $OID_NUM_PQOS_CORE_PROP updates

=cut

sub data_update_core_prop {
	my $ret = 0;

	%data_core_prop = ();
	for (
		my $cpu_id = 0;
		defined $l3ca_cap_p && $cpu_id < $num_cores;
		$cpu_id++
		) {
		($ret, my $cos_id) = pqos::pqos_alloc_assoc_get($cpu_id);
		if (0 != $ret) {
			last;
		}

		($ret, my $socket_id) =
			pqos::pqos_cpu_get_socketid($cpuinfo_p, $cpu_id);
		if (0 != $ret) {
			last;
		}

		my $oid = NetSNMP::OID->new(
			"$OID_NUM_PQOS_CORE_PROP.$socket_id.$cpu_id.$CORE_COS_ID");
		$data_core_prop{$oid} = $cos_id;
	}

	if (defined $llc_mon_data_p_a) {
		if (0 != pqos::pqos_mon_poll($llc_mon_data_p_a, $num_cores)) {
			print __LINE__, " pqos::pqos_mon_poll FAILED!\n";
			return;
		}
	}

	for (
		my $cpu_id = 0;
		defined $llc_mon_data_p_a && $cpu_id < $num_cores;
		$cpu_id++
		) {
		($ret, my $socket_id) =
			pqos::pqos_cpu_get_socketid($cpuinfo_p, $cpu_id);
		if (0 != $ret) {
			last;
		}

		my $llc_mon_data_p =
			pqos::pqos_mon_data_p_a_getitem($llc_mon_data_p_a, $cpu_id);

		if (defined $llc_mon_data_p) {
			my $oid = NetSNMP::OID->new(
				"$OID_NUM_PQOS_CORE_PROP.$socket_id.$cpu_id.$CORE_CMT_LLC_ID");
			$data_core_prop{$oid} =
				pqos::pqos_mon_data_p_value($llc_mon_data_p)->{values}->{llc};
		}
	}

	@data_core_prop_keys =
		sort {NetSNMP::OID->new($a) <=> NetSNMP::OID->new($b)}
		keys %data_core_prop;
}

=item data_update_cos()

Handles $OID_NUM_PQOS_COS updates

=cut

sub data_update_cos {
	%data_cos = ();
	my $l3ca = pqos::pqos_l3ca->new();

	for (
		my $socket_idx = 0;
		defined $l3ca_cap_p && $socket_idx < $num_sockets;
		$socket_idx++
		) {
		my $socket_id = pqos::uint_a_getitem($sockets_a, $socket_idx);
		for (my $cos_id = 0; $cos_id < $num_cos; $cos_id++) {
			if (0 != pqos::get_l3ca($l3ca, $socket_id, $cos_id)) {
				last;
			}

			my $oid_num = "$OID_NUM_PQOS_COS.$socket_id.$l3ca->{class_id}";
			$data_cos{new NetSNMP::OID("$oid_num.$COS_CDP_ID")} =
				int($l3ca->{cdp});

			if (1 == int($l3ca->{cdp})) {
				$data_cos{new NetSNMP::OID("$oid_num.$COS_DATA_MASK_ID")} =
					int($l3ca->{u}->{s}->{data_mask});
				$data_cos{new NetSNMP::OID("$oid_num.$COS_CODE_MASK_ID")} =
					int($l3ca->{u}->{s}->{code_mask});
			} else {
				$data_cos{new NetSNMP::OID("$oid_num.$COS_WAYS_MASK_ID")} =
					int($l3ca->{u}->{ways_mask});
			}
		}
	}

	@data_cos_keys =
		sort {NetSNMP::OID->new($a) <=> NetSNMP::OID->new($b)} keys %data_cos;
}

=item data_update_mon_ctrl()

Handles $OID_NUM_PQOS_MON_CTRL updates

=cut

sub data_update_mon_ctrl {
	%data_mon_ctrl = ();

	my $oid =
		NetSNMP::OID->new(
		"$OID_NUM_PQOS_MON_CTRL.$MON_CMT_ID.$MON_LLC_SUPP_ID");
	$data_mon_ctrl{$oid} = defined $llc_mon_supp ? 1 : 0;

	if (defined $llc_mon_supp) {
		$oid =
			NetSNMP::OID->new(
			"$OID_NUM_PQOS_MON_CTRL.$MON_CMT_ID.$MON_LLC_STATE_ID");
		$data_mon_ctrl{$oid} = defined $llc_mon_data_p_a ? 1 : 0;
	}

	@data_mon_ctrl_keys =
		sort {NetSNMP::OID->new($a) <=> NetSNMP::OID->new($b)}
		keys %data_mon_ctrl;
}

=item is_whole_number()

 Arguments:
    $number: number to be tested

 Returns: true if number is non-negative integer

Test if number is non-negative integer.

=cut

sub is_whole_number {
	my ($number) = @_;
	return $number =~ /^\d+$/;
}

=item bits_count()

 Arguments:
    $bitmask: bitmask to be checked for set bits

 Returns: number of set bits

Counts number of set bits in a bitmask.

=cut

sub bits_count {
	my ($bitmask) = @_;
	my $count = 0;

	for (; $bitmask != 0; $count++) {
		$bitmask &= $bitmask - 1;
	}

	return $count;
}

=item is_contiguous()

 Arguments:
    $bitmask: bitmask to be tested

 Returns: true if bitmask is contiguous

Test if bitmask is contiguous.

=cut

sub is_contiguous {
	my ($bitmask) = @_;
	Readonly my $MAX_IDX => 64;
	my $j = 0;

	if ($bitmask && (2**$MAX_IDX - 1) == 0) {
		return 0;
	}

	for (my $i = 0; $i < $MAX_IDX; $i++) {
		if (((1 << $i) & $bitmask) != 0) {
			$j++;
		} elsif ($j > 0) {
			last;
		}
	}

	if (bits_count($bitmask) != $j) {
		return 0;
	}

	return 1;
}

=item handle_core_prop()

 Arguments:
    $request, $request_info: request related data

Handle $OID_NUM_PQOS_CORE_PROP OID requests

=cut

sub handle_core_prop {
	my ($request, $request_info) = @_;
	my $oid  = $request->getOID();
	my $mode = $request_info->getMode();

	if (MODE_GET == $mode) {
		if (exists $data_core_prop{$oid}) {
			$request->setValue(ASN_UNSIGNED, $data_core_prop{$oid});
		}

		return;
	}

	if (MODE_GETNEXT == $mode) {
		foreach (@data_core_prop_keys) {
			$_ = NetSNMP::OID->new($_);
			if ($oid < $_) {
				$request->setOID($_);
				$request->setValue(ASN_UNSIGNED, $data_core_prop{$_});
				last;
			}
		}

		return;
	}

	if (MODE_SET_RESERVE1 == $mode) {
		my $value = $request->getValue();
		if (!exists $data_core_prop{$oid}) {
			$request->setError($request_info, SNMP_ERR_NOSUCHNAME);
			return;
		}

		my $field_id = ($oid->to_array())[$oid->length - 1];

		if ($field_id != $CORE_COS_ID) {
			$request->setError($request_info, SNMP_ERR_READONLY);
			return;
		}

		if (!is_whole_number($value)) {
			$request->setError($request_info, SNMP_ERR_WRONGTYPE);
		} elsif ($value < 0 || $value >= $num_cos) {
			$request->setError($request_info, SNMP_ERR_WRONGVALUE);
		}

		return;
	}

	if (MODE_SET_ACTION == $mode) {
		my $cpu_id = ($oid->to_array())[$oid->length - 2];
		my $cos_id = $request->getValue();
		if (0 != pqos::pqos_alloc_assoc_set($cpu_id, $cos_id)) {
			$request->setError($request_info, SNMP_ERR_GENERR);
		}
	}

	return;
}

=item handle_cos()

 Arguments:
    $request, $request_info: request related data

Handle $OID_NUM_PQOS_COS OID requests

=cut

sub handle_cos {
	my ($request, $request_info) = @_;
	my $oid  = $request->getOID();
	my $mode = $request_info->getMode();

	if (MODE_GET == $mode) {
		if (exists $data_cos{$oid}) {
			$request->setValue(ASN_COUNTER64, $data_cos{$oid});
		}
		return;
	}

	if (MODE_GETNEXT == $mode) {
		foreach (@data_cos_keys) {
			$_ = NetSNMP::OID->new($_);
			if ($oid < $_) {
				$request->setOID($_);
				$request->setValue(ASN_COUNTER64, $data_cos{$_});
				last;
			}
		}
		return;
	}

	if (MODE_SET_RESERVE1 == $mode) {
		my $value = $request->getValue();
		if (!exists $data_cos{$oid}) {
			$request->setError($request_info, SNMP_ERR_NOSUCHNAME);
			return;
		}

		my $field_id = ($oid->to_array())[$oid->length - 1];

		if ($field_id != $COS_WAYS_MASK_ID &&
			$field_id != $COS_DATA_MASK_ID &&
			$field_id != $COS_CODE_MASK_ID) {
			$request->setError($request_info, SNMP_ERR_READONLY);
			return;
		}

		if (!is_whole_number($value)) {
			$request->setError($request_info, SNMP_ERR_WRONGTYPE);
			return;
		}

		if ($value <= 0 || $value >= (2**$num_ways)) {
			$request->setError($request_info, SNMP_ERR_WRONGVALUE);
			return;
		}

		if (!is_contiguous($value)) {
			$request->setError($request_info, SNMP_ERR_WRONGVALUE);
			return;
		}

		return;
	}

	if (MODE_SET_ACTION == $mode) {
		my $value     = $request->getValue();
		my $socket_id = ($oid->to_array())[$oid->length - 3];
		my $cos_id    = ($oid->to_array())[$oid->length - 2];
		my $field_id  = ($oid->to_array())[$oid->length - 1];
		my $l3ca      = pqos::pqos_l3ca->new();

		if (0 != pqos::get_l3ca($l3ca, $socket_id, $cos_id)) {
			$request->setError($request_info, SNMP_ERR_GENERR);
			return;
		}

		if ($field_id == $COS_WAYS_MASK_ID) {
			$l3ca->{u}->{ways_mask} = $value;
		} elsif ($field_id == $COS_DATA_MASK_ID) {
			$l3ca->{u}->{s}->{data_mask} = $value;
		} elsif ($field_id == $COS_CODE_MASK_ID) {
			$l3ca->{u}->{s}->{code_mask} = $value;
		}

		if (0 != pqos::pqos_l3ca_set($socket_id, 1, $l3ca)) {
			$request->setError($request_info, SNMP_ERR_GENERR);
		}
	}

	return;
}

=item handle_mon_ctrl()

 Arguments:
    $request, $request_info: request related data

Handle $OID_NUM_PQOS_MON_CTRL OID requests

=cut

sub handle_mon_ctrl {
	my ($request, $request_info) = @_;
	my $oid  = $request->getOID();
	my $mode = $request_info->getMode();

	if (MODE_GET == $mode) {
		if (exists $data_mon_ctrl{$oid}) {
			$request->setValue(ASN_UNSIGNED, $data_mon_ctrl{$oid});
		}

		return;
	}

	if (MODE_GETNEXT == $mode) {
		foreach (@data_mon_ctrl_keys) {
			$_ = NetSNMP::OID->new($_);
			if ($oid < $_) {
				$request->setOID($_);
				$request->setValue(ASN_UNSIGNED, $data_mon_ctrl{$_});
				last;
			}
		}

		return;
	}

	if (MODE_SET_RESERVE1 == $mode) {
		my $mon_tech_id = ($oid->to_array())[$oid->length - 2];
		my $field_id    = ($oid->to_array())[$oid->length - 1];
		my $value       = $request->getValue();
		if (!exists $data_mon_ctrl{$oid}) {
			$request->setError($request_info, SNMP_ERR_NOSUCHNAME);
			return;
		}

		if ($mon_tech_id == $MON_CMT_ID && $field_id != $MON_LLC_STATE_ID) {
			$request->setError($request_info, SNMP_ERR_READONLY);
			return;
		}

		if (!is_whole_number($value)) {
			$request->setError($request_info, SNMP_ERR_WRONGTYPE);
		} elsif ($value < 0 || $value > 1) {
			$request->setError($request_info, SNMP_ERR_WRONGVALUE);
		}

		return;
	}

	if (MODE_SET_ACTION == $mode) {
		my $mon_tech_id = ($oid->to_array())[$oid->length - 2];
		my $field_id    = ($oid->to_array())[$oid->length - 1];
		my $value       = $request->getValue();

		if ($mon_tech_id == $MON_CMT_ID && $field_id == $MON_LLC_STATE_ID) {
			if ($value == 0) {
				cmt_llc_stop();
				if (defined $llc_mon_data_p_a) {
					$request->setError($request_info, SNMP_ERR_GENERR);
				}
			} else {
				cmt_llc_start();
				if (!defined $llc_mon_data_p_a) {
					$request->setError($request_info, SNMP_ERR_GENERR);
				}
			}
		}
	}

	return;
}

=item cmt_llc_start()

Subroutine to start LLC monitoring

=cut

sub cmt_llc_start {
	if (defined $llc_mon_data_p_a) {
		print __LINE__, " LLC monitoring started already!\n";
		return;
	}

	print "Starting LLC monitoring...\n";

	my $monitor_p_p = pqos::new_pqos_monitor_p_p();
	if (
		0 != pqos::pqos_cap_get_event(
			$cap_p, $pqos::PQOS_MON_EVENT_L3_OCCUP, $monitor_p_p)
		) {
		print __LINE__, " LLC monitoring not supported!\n";
		pqos::delete_pqos_monitor_p_p($monitor_p_p);
		return;
	}

	$llc_mon_data_p_a = pqos::new_pqos_mon_data_p_a($num_cores);
	for (my $cpu_id = 0; $cpu_id < $num_cores; $cpu_id++) {
		pqos::pqos_mon_data_p_a_setitem($llc_mon_data_p_a, $cpu_id, undef);
	}

	my $cpu_id_p = pqos::new_uintp();

	for (my $cpu_id = 0; $cpu_id < $num_cores; $cpu_id++) {
		pqos::uintp_assign($cpu_id_p, $cpu_id);

		my $llc_mon_data_p = pqos::new_pqos_mon_data_p();
		if (
			0 != pqos::pqos_mon_start(
				1, $cpu_id_p, $pqos::PQOS_MON_EVENT_L3_OCCUP,
				undef, $llc_mon_data_p)
			) {
			print __LINE__, " pqos::pqos_mon_start for core ", $cpu_id,
				" FAILED!\n";
			pqos::delete_pqos_mon_data_p($llc_mon_data_p);
			cmt_llc_stop();
			last;
		}
		pqos::pqos_mon_data_p_a_setitem($llc_mon_data_p_a, $cpu_id,
			$llc_mon_data_p);
	}

	if (defined $cpu_id_p) {
		pqos::delete_uintp($cpu_id_p);
	}

	if (defined $monitor_p_p) {
		pqos::delete_pqos_monitor_p_p($monitor_p_p);
	}
}

=item cmt_llc_stop()

Subroutine to stop LLC monitoring

=cut

sub cmt_llc_stop {
	if (!defined $llc_mon_data_p_a) {
		print __LINE__, " LLC monitoring not started!\n";
		return;
	}

	print "Stopping LLC monitoring...\n";

	for (my $cpu_id = 0; $cpu_id < $num_cores; $cpu_id++) {
		my $llc_mon_data_p =
			pqos::pqos_mon_data_p_a_getitem($llc_mon_data_p_a, $cpu_id);
		if (defined $llc_mon_data_p) {
			if (0 != pqos::pqos_mon_stop($llc_mon_data_p)) {
				print __LINE__, " pqos::pqos_mon_stop for core ", $cpu_id,
					" FAILED!\n";
			}
			pqos::delete_pqos_mon_data_p($llc_mon_data_p);
		}
	}

	pqos::delete_pqos_mon_data_p_a($llc_mon_data_p_a);
	$llc_mon_data_p_a = undef;
}

=item handle_snmp_req()

 Arguments:
    $handler, $registration_info, ... : request related data

Subroutine registered as a request handler, calls appropriate handlers

=cut

sub handle_snmp_req {
	my ($handler, $registration_info, $request_info, $requests) = @_;

	data_update;

	my $root_oid = $registration_info->getRootOID();

	for (my $request = $requests; $request; $request = $request->next()) {
		if (MODE_SET_COMMIT == $request_info->getMode()) {
			$data_timestamp = 0;
		} elsif ($root_oid == $oid_pqos_cos) {
			handle_cos($request, $request_info);
		} elsif ($root_oid == $oid_pqos_core_prop) {
			handle_core_prop($request, $request_info);
		} elsif ($root_oid == $oid_pqos_mon_ctrl) {
			handle_mon_ctrl($request, $request_info);
		}
	}

	return;
}

if (0 == pqos_init) {
	local $SIG{'INT'}  = \&shutdown_agent;
	local $SIG{'QUIT'} = \&shutdown_agent;

	my $agent = NetSNMP::agent->new('Name' => "pqos", 'AgentX' => 1);

	$oid_pqos_cos       = NetSNMP::OID->new($OID_NUM_PQOS_COS);
	$oid_pqos_core_prop = NetSNMP::OID->new($OID_NUM_PQOS_CORE_PROP);
	$oid_pqos_mon_ctrl  = NetSNMP::OID->new($OID_NUM_PQOS_MON_CTRL);

	$agent->register("pqos_cos", $oid_pqos_cos, \&handle_snmp_req) or
		die "registration of oid_pqos_cos handler failed!\n";

	$agent->register("pqos_core_prop", $oid_pqos_core_prop, \&handle_snmp_req)
		or
		die "registration of oid_pqos_core_prop handler failed!\n";

	$agent->register("pqos_mon_ctrl", $oid_pqos_mon_ctrl, \&handle_snmp_req) or
		die "registration of oid_pqos_mon_ctrl handler failed!\n";

	while ($active) {
		$agent->agent_check_and_process(1);
	}

	$agent->shutdown();
}
