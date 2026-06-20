package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"reconciliation/handler"
	"reconciliation/model"
	"reconciliation/service"
)

func main() {
	port := flag.String("port", "8080", "HTTP server port")
	flag.Parse()

	if envPort := os.Getenv("PORT"); envPort != "" {
		port = &envPort
	}

	cfg := &model.ReconciliationConfig{
		Wechat: &model.WechatConfig{
			AppID:    os.Getenv("WECHAT_APP_ID"),
			MchID:    os.Getenv("WECHAT_MCH_ID"),
			APIv3Key: os.Getenv("WECHAT_API_V3_KEY"),
			CertPath: os.Getenv("WECHAT_CERT_PATH"),
			KeyPath:  os.Getenv("WECHAT_KEY_PATH"),
		},
		Alipay: &model.AlipayConfig{
			AppID:      os.Getenv("ALIPAY_APP_ID"),
			PrivateKey: os.Getenv("ALIPAY_PRIVATE_KEY"),
			PublicKey:  os.Getenv("ALIPAY_PUBLIC_KEY"),
		},
	}

	svc := service.NewReconciliationService(cfg)
	h := handler.NewHandler(svc)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/api/wechat/bill/parse", h.ParseWechatBill)
	mux.HandleFunc("/api/alipay/bill/parse", h.ParseAlipayBill)
	mux.HandleFunc("/api/reconcile", h.Reconcile)
	mux.HandleFunc("/api/reconcile/detail", h.ReconcileDetail)

	addr := fmt.Sprintf(":%s", *port)
	log.Printf("Server starting on %s", addr)
	log.Printf("Endpoints:")
	log.Printf("  GET  /health")
	log.Printf("  POST /api/wechat/bill/parse")
	log.Printf("  POST /api/alipay/bill/parse")
	log.Printf("  POST /api/reconcile")
	log.Printf("  POST /api/reconcile/detail")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
