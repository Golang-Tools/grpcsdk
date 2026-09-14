package grpcsdk

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// certFiles 测试用的证书文件路径集合
type certFiles struct {
	// ca 根证书文件
	ca string
	// serverCert 服务端证书文件
	serverCert string
	// serverKey 服务端私钥文件
	serverKey string
	// clientCert 客户端证书文件
	clientCert string
	// clientKey 客户端私钥文件
	clientKey string
	// caPool 根证书池
	caPool *x509.CertPool
}

// writePEMFile 把DER数据以PEM格式写入文件
// @params t *testing.T 测试对象
// @params path string 文件路径
// @params blockType string PEM块类型
// @params der []byte DER数据
func writePEMFile(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create pem file error: %v", err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		t.Fatalf("encode pem file error: %v", err)
	}
}

// writeECKeyFile 把ECDSA私钥以PEM格式写入文件
// @params t *testing.T 测试对象
// @params path string 文件路径
// @params key *ecdsa.PrivateKey 私钥
func writeECKeyFile(t *testing.T, path string, key *ecdsa.PrivateKey) {
	t.Helper()
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal ec key error: %v", err)
	}
	writePEMFile(t, path, "EC PRIVATE KEY", der)
}

// generateTestCerts 生成测试用的CA、服务端与客户端证书
// @params t *testing.T 测试对象
// @returns certFiles 证书文件路径集合
func generateTestCerts(t *testing.T) certFiles {
	t.Helper()
	dir := t.TempDir()
	notBefore := time.Now().Add(-time.Hour)
	notAfter := time.Now().Add(24 * time.Hour)

	// 根证书
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ca key error: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "grpcsdk-test-ca"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create ca cert error: %v", err)
	}
	caFile := filepath.Join(dir, "ca.pem")
	writePEMFile(t, caFile, "CERTIFICATE", caDER)
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})) {
		t.Fatal("append ca cert error")
	}

	// 服务端证书,证书需要包含127.0.0.1,否则握手时校验不过
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate server key error: %v", err)
	}
	serverTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "grpcsdk-test-server"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTmpl, caTmpl, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create server cert error: %v", err)
	}
	serverCertFile := filepath.Join(dir, "server.pem")
	serverKeyFile := filepath.Join(dir, "server-key.pem")
	writePEMFile(t, serverCertFile, "CERTIFICATE", serverDER)
	writeECKeyFile(t, serverKeyFile, serverKey)

	// 客户端证书
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate client key error: %v", err)
	}
	clientTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "grpcsdk-test-client"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTmpl, caTmpl, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create client cert error: %v", err)
	}
	clientCertFile := filepath.Join(dir, "client.pem")
	clientKeyFile := filepath.Join(dir, "client-key.pem")
	writePEMFile(t, clientCertFile, "CERTIFICATE", clientDER)
	writeECKeyFile(t, clientKeyFile, clientKey)

	return certFiles{
		ca:         caFile,
		serverCert: serverCertFile,
		serverKey:  serverKeyFile,
		clientCert: clientCertFile,
		clientKey:  clientKeyFile,
		caPool:     caPool,
	}
}

// TestSDKInitWithTLS 验证配置CA证书后使用TLS连接
func TestSDKInitWithTLS(t *testing.T) {
	certs := generateTestCerts(t)
	serverCert, err := tls.LoadX509KeyPair(certs.serverCert, certs.serverKey)
	if err != nil {
		t.Fatalf("load server cert error: %v", err)
	}
	addr, cleanup := startHealthServer(t, grpc.Creds(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{serverCert},
		MinVersion:   tls.VersionTLS12,
	})))
	defer cleanup()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(WithQueryAddresses(addr), WithCaCertPath(certs.ca)); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()
	cli, release := sdk.GetClient()
	defer release()
	if _, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
}

// TestSDKInitWithMTLS 验证同时配置客户端证书与私钥时使用mTLS连接
func TestSDKInitWithMTLS(t *testing.T) {
	certs := generateTestCerts(t)
	serverCert, err := tls.LoadX509KeyPair(certs.serverCert, certs.serverKey)
	if err != nil {
		t.Fatalf("load server cert error: %v", err)
	}
	addr, cleanup := startHealthServer(t, grpc.Creds(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certs.caPool,
		MinVersion:   tls.VersionTLS12,
	})))
	defer cleanup()

	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	if err := sdk.Init(
		WithQueryAddresses(addr),
		WithCaCertPath(certs.ca),
		WithClientCertPath(certs.clientCert),
		WithClientKeyPath(certs.clientKey),
	); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	defer sdk.Close()
	cli, release := sdk.GetClient()
	defer release()
	if _, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check error: %v", err)
	}
}

// TestSDKInitWithInvalidCA 验证CA文件内容非法时的错误
func TestSDKInitWithInvalidCA(t *testing.T) {
	badCA := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(badCA, []byte("not a pem file"), 0o600); err != nil {
		t.Fatalf("write bad ca file error: %v", err)
	}
	sdk := New(healthFactory(), &healthpb.Health_ServiceDesc)
	err := sdk.Init(WithQueryAddresses("127.0.0.1:1"), WithCaCertPath(badCA))
	if !errors.Is(err, ErrCACertNotParsed) {
		t.Fatalf("期待ErrCACertNotParsed, got: %v", err)
	}
}
