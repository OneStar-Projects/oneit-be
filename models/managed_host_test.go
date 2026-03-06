package models

import (
	"testing"
)

func TestManagedHostBasic(t *testing.T) {
	// 这是一个基本的测试，验证ManagedHost结构体定义是否正确
	host := &ManagedHost{
		HostIdent:     "test-host-001",
		SSHIp:         "192.168.1.100",
		SSHPort:       22,
		SSHUser:       "root",
		AuthMethod:    "password",
		CredentialRef: "cred-001",
		Status:        "pending",
		Note:          "Test host",
		SudoRequired:  false,
	}

	// 验证基本字段
	if host.HostIdent != "test-host-001" {
		t.Errorf("Expected HostIdent to be 'test-host-001', got %s", host.HostIdent)
	}

	if host.SSHIp != "192.168.1.100" {
		t.Errorf("Expected SSHIp to be '192.168.1.100', got %s", host.SSHIp)
	}

	if host.SSHPort != 22 {
		t.Errorf("Expected SSHPort to be 22, got %d", host.SSHPort)
	}

	if host.Status != "pending" {
		t.Errorf("Expected Status to be 'pending', got %s", host.Status)
	}
}

func TestHostAgentBasic(t *testing.T) {
	// 这是一个基本的测试，验证HostAgent结构体定义是否正确
	hostAgent := &HostAgent{
		HostID:       1,
		ComponentID:  1,
		Status:       "pending",
		ConfigData:   `{"key": "value"}`,
		ErrorMessage: "",
	}

	// 验证基本字段
	if hostAgent.HostID != 1 {
		t.Errorf("Expected HostID to be 1, got %d", hostAgent.HostID)
	}

	if hostAgent.ComponentID != 1 {
		t.Errorf("Expected ComponentID to be 1, got %d", hostAgent.ComponentID)
	}

	if hostAgent.Status != "pending" {
		t.Errorf("Expected Status to be 'pending', got %s", hostAgent.Status)
	}

	if hostAgent.ConfigData != `{"key": "value"}` {
		t.Errorf("Expected ConfigData to be '{\"key\": \"value\"}', got %s", hostAgent.ConfigData)
	}
}
