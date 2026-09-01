package controller

import (
	"testing"

	kubevirtv1 "kubevirt.io/api/core/v1"
)

func TestInstancePhaseFromVMRunning(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{}
	vm.Status.PrintableStatus = kubevirtv1.VirtualMachineStatusRunning
	if got := instancePhaseFromVM(vm); got != instancePhaseRunning {
		t.Fatalf("got %q", got)
	}
}

func TestPreferGuestIPPublicNic(t *testing.T) {
	ip := preferGuestIP([]kubevirtv1.VirtualMachineInstanceNetworkInterface{
		{Name: "default", IP: "10.233.1.5"},
		{Name: "public", IP: "10.0.50.12"},
	})
	if ip != "10.0.50.12" {
		t.Fatalf("got %q", ip)
	}
}
