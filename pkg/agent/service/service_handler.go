package service

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/bhangun/mandau/api/v1"
	"github.com/bhangun/mandau/plugins/services/firewall"
	"github.com/bhangun/mandau/plugins/services/nginx"
	"github.com/bhangun/mandau/plugins/services/systemd"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ServicesHandler struct {
	v1.UnimplementedNginxServiceServer
	v1.UnimplementedSystemdServiceServer
	v1.UnimplementedFirewallServiceServer
	v1.UnimplementedACMEServiceServer
	v1.UnimplementedHostEnvironmentServiceServer
	v1.UnimplementedServiceDeploymentServiceServer

	serviceMgr *ServiceManager
}

func NewServicesHandler(serviceMgr *ServiceManager) *ServicesHandler {
	return &ServicesHandler{
		serviceMgr: serviceMgr,
	}
}

// Nginx Service Handlers
func (h *ServicesHandler) CreateVirtualHost(ctx context.Context, req *v1.CreateVirtualHostRequest) (*v1.CreateVirtualHostResponse, error) {
	vhost := &nginx.VirtualHost{
		ServerName: req.ServerName,
		Listen:     int(req.Listen),
		Root:       req.Root,
		Index:      req.Index,
		ProxyPass:  req.ProxyPass,
	}

	// Convert locations
	for _, loc := range req.Locations {
		vhost.Locations = append(vhost.Locations, nginx.Location{
			Path:      loc.Path,
			ProxyPass: loc.ProxyPass,
			Root:      loc.Root,
			TryFiles:  loc.TryFiles,
			Headers:   loc.Headers,
		})
	}

	// Convert SSL config
	if req.Ssl != nil {
		vhost.SSL = &nginx.SSLConfig{
			Certificate:    req.Ssl.Certificate,
			CertificateKey: req.Ssl.CertificateKey,
			Protocols:      req.Ssl.Protocols,
			Ciphers:        req.Ssl.Ciphers,
		}
	}

	if err := h.serviceMgr.nginx.CreateVirtualHost(vhost); err != nil {
		return nil, status.Errorf(codes.Internal, "create vhost: %v", err)
	}

	return &v1.CreateVirtualHostResponse{
		Status: "success",
	}, nil
}

func (h *ServicesHandler) CreateReverseProxy(ctx context.Context, req *v1.CreateReverseProxyRequest) (*v1.CreateReverseProxyResponse, error) {
	err := h.serviceMgr.Nginx().CreateReverseProxy(
		req.Domain,
		req.Upstream,
		int(req.Port),
	)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "create reverse proxy: %v", err)
	}

	return &v1.CreateReverseProxyResponse{
		Status: "success",
	}, nil
}

// Systemd Service Handlers
func (h *ServicesHandler) CreateService(ctx context.Context, req *v1.CreateServiceRequest) (*v1.CreateServiceResponse, error) {
	service := &systemd.ServiceUnit{
		Name:          req.Name,
		Description:   req.Description,
		After:         req.After,
		Type:          req.Type,
		User:          req.User,
		Group:         req.Group,
		WorkingDir:    req.WorkingDir,
		ExecStart:     req.ExecStart,
		ExecStop:      req.ExecStop,
		Environment:   req.Environment,
		Restart:       req.Restart,
		RestartSec:    int(req.RestartSec),
		LimitNOFILE:   int(req.LimitNofile),
		MemoryLimit:   req.MemoryLimit,
		PrivateTmp:    req.PrivateTmp,
		ProtectSystem: req.ProtectSystem,
	}

	if err := h.serviceMgr.systemd.CreateService(service); err != nil {
		return nil, status.Errorf(codes.Internal, "create service: %v", err)
	}

	return &v1.CreateServiceResponse{
		Status: "success",
	}, nil
}

func (h *ServicesHandler) StartService(ctx context.Context, req *v1.StartServiceRequest) (*v1.StartServiceResponse, error) {
	if err := h.serviceMgr.Systemd().StartService(req.Name); err != nil {
		return nil, status.Errorf(codes.Internal, "start service: %v", err)
	}

	return &v1.StartServiceResponse{
		Status: "success",
	}, nil
}

func (h *ServicesHandler) GetServiceStatus(ctx context.Context, req *v1.GetServiceStatusRequest) (*v1.GetServiceStatusResponse, error) {
	svcStatus, err := h.serviceMgr.Systemd().GetServiceStatus(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get status: %v", err)
	}

	return &v1.GetServiceStatusResponse{
		Status: svcStatus,
	}, nil
}

// Firewall Handlers
func (h *ServicesHandler) AddRule(ctx context.Context, req *v1.AddFirewallRuleRequest) (*v1.AddFirewallRuleResponse, error) {
	rule := &firewall.FirewallRule{
		Action:   req.Action,
		Proto:    req.Proto,
		FromIP:   req.FromIp,
		FromPort: int(req.FromPort),
		ToIP:     req.ToIp,
		ToPort:   int(req.ToPort),
		Comment:  req.Comment,
	}

	if err := h.serviceMgr.firewall.AddRule(rule); err != nil {
		return nil, status.Errorf(codes.Internal, "add rule: %v", err)
	}

	return &v1.AddFirewallRuleResponse{
		Status: "success",
	}, nil
}

func (h *ServicesHandler) AllowPort(ctx context.Context, req *v1.AllowPortRequest) (*v1.AllowPortResponse, error) {
	if err := h.serviceMgr.Firewall().AllowPort(int(req.Port), req.Proto); err != nil {
		return nil, status.Errorf(codes.Internal, "allow port: %v", err)
	}

	return &v1.AllowPortResponse{
		Status: "success",
	}, nil
}

// ACME Handlers
func (h *ServicesHandler) ObtainCertificate(ctx context.Context, req *v1.ObtainCertificateRequest) (*v1.ObtainCertificateResponse, error) {
	cert, err := h.serviceMgr.ACME().ObtainCertificate(req.Domain)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "obtain certificate: %v", err)
	}

	return &v1.ObtainCertificateResponse{
		Certificate: &v1.Certificate{
			Domain:    cert.Domain,
			CertPath:  cert.CertPath,
			KeyPath:   cert.KeyPath,
			ExpiresAt: cert.ExpiresAt,
		},
	}, nil
}

func (h *ServicesHandler) RenewAll(ctx context.Context, req *v1.RenewAllCertificatesRequest) (*v1.RenewAllCertificatesResponse, error) {
	if err := h.serviceMgr.ACME().RenewAllCertificates(); err != nil {
		return nil, status.Errorf(codes.Internal, "renew all: %v", err)
	}

	return &v1.RenewAllCertificatesResponse{
		Status: "success",
	}, nil
}

// Host Environment Handlers
func (h *ServicesHandler) GetHostInfo(ctx context.Context, req *v1.GetHostInfoRequest) (*v1.GetHostInfoResponse, error) {
	info, err := h.serviceMgr.Environment().GetHostInfo()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get host info: %v", err)
	}

	return &v1.GetHostInfoResponse{
		Hostname:     info.Hostname,
		Os:           info.OS,
		Kernel:       info.Kernel,
		Architecture: info.Architecture,
		CpuCores:     int32(info.CPUCores),
		MemoryMb:     info.MemoryMB,
		DiskGb:       info.DiskGB,
		Uptime:       info.Uptime,
	}, nil
}

func (h *ServicesHandler) InstallPackage(ctx context.Context, req *v1.InstallPackageRequest) (*v1.InstallPackageResponse, error) {
	if err := h.serviceMgr.Environment().InstallPackage(req.PackageName); err != nil {
		return nil, status.Errorf(codes.Internal, "install package: %v", err)
	}

	return &v1.InstallPackageResponse{
		Status: "success",
	}, nil
}

// Complete Service Deployment Handler
func (h *ServicesHandler) DeployWebService(req *v1.DeployWebServiceRequest, stream v1.ServiceDeploymentService_DeployWebServiceServer) error {
	ctx := stream.Context()

	// Send initial event
	stream.Send(&v1.ServiceOperationEvent{
		OperationId: generateOperationID(),
		State:       "RUNNING",
		Message:     "Starting web service deployment",
	})

	config := &WebServiceConfig{
		Name:        req.Name,
		Description: req.Description,
		Domain:      req.Domain,
		Port:        int(req.Port),
		Command:     req.Command,
		WorkingDir:  req.WorkingDir,
		User:        req.User,
		SSL:         req.Ssl,
		Environment: req.Environment,
	}

	// Stream progress updates
	if err := h.serviceMgr.DeployWebService(ctx, config); err != nil {
		stream.Send(&v1.ServiceOperationEvent{
			State: "FAILED",
			Error: err.Error(),
		})
		return status.Errorf(codes.Internal, "deploy failed: %v", err)
	}

	stream.Send(&v1.ServiceOperationEvent{
		State:   "COMPLETED",
		Message: "Web service deployed successfully",
	})

	return nil
}

// generateOperationID generates a unique operation ID
func generateOperationID() string {
	return fmt.Sprintf("op-%d", time.Now().UnixNano())
}
