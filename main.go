package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

// WebSocket 反向代理
func wsReverseProxy(target string) http.Handler {
	targetURL, err := url.Parse(target)
	if err != nil {
		log.Fatal("Invalid target URL:", err)
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetConn, _, err := websocket.DefaultDialer.Dial(targetURL.String(), nil)
		if err != nil {
			log.Println("Dial error:", err)
			http.Error(w, "Failed to dial backend", http.StatusInternalServerError)
			return
		}
		defer targetConn.Close()

		clientConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Upgrade error:", err)
			return
		}
		defer clientConn.Close()

		go func() {
			for {
				mt, msg, err := clientConn.ReadMessage()
				if err != nil {
					log.Println("Client read error:", err)
					return
				}
				if err := targetConn.WriteMessage(mt, msg); err != nil {
					log.Println("Target write error:", err)
					return
				}
			}
		}()

		for {
			mt, msg, err := targetConn.ReadMessage()
			if err != nil {
				log.Println("Target read error:", err)
				return
			}
			if err := clientConn.WriteMessage(mt, msg); err != nil {
				log.Println("Client write error:", err)
				return
			}
		}
	})
}

// HTTP 反向代理
func httpReverseProxy(target string) http.Handler {
	targetURL, err := url.Parse(target)
	if err != nil {
		log.Fatal("Invalid target URL:", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// 路由处理
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fmt.Fprintln(w, "Hello world")
			return
		}

		if strings.HasPrefix(r.URL.Path, "/amd06ws") {
			wsProxy := wsReverseProxy("ws://npm2amd06ws.p.dnsabr.com")
			wsProxy.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/azusxh") {
			httpProxy := httpReverseProxy("https://ex01.choreoapps.dev")
			httpProxy.ServeHTTP(w, r)
			return
		}

		http.NotFound(w, r)
	})

	// 启动服务器
	log.Println("Server listening on :" + port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("ListenAndServe error:", err)
	}
}
