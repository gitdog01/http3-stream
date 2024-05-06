// main.go
package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/pion/webrtc/v3"
	"github.com/quic-go/quic-go/http3"
)

const (
	certFile = "server.crt"
	keyFile  = "server.key"
)

var (
	// WebRTC Configuration
	webrtcConfig = webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
)

type WebRTCAnswer struct {
	SDP  string `json:"sdp"`
	Type string `json:"type"`
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "Welcome to HTTP/3 Screen Sharing Server!")
	})

	http.HandleFunc("/offer", handleOffer)

	// Load TLS certificate
	tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatalf("failed to load TLS certificate: %v", err)
	}

	// Create an HTTP/3 server instance
	server := http3.Server{
		Addr: ":4433",
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			NextProtos:   []string{"h3", "http/1.1"}, // Support both HTTP/3 and HTTP/1.1
		},
	}

	// Start the HTTP/3 server
	log.Printf("Starting HTTP/3 server on https://localhost:4433")
	log.Fatal(server.ListenAndServeTLS(certFile, keyFile))
}

func handleOffer(w http.ResponseWriter, r *http.Request) {
	// Read and decode the SDP offer
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	offer := webrtc.SessionDescription{}
	if err := json.Unmarshal(body, &offer); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create a new WebRTC Peer Connection
	peerConnection, err := webrtc.NewPeerConnection(webrtcConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer peerConnection.Close()

	// Handle incoming track
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("Received track: %s, SSRC: %d", track.Kind().String(), track.SSRC())
	})

	// Set the remote SDP offer
	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create SDP answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := peerConnection.SetLocalDescription(answer); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return SDP answer as JSON
	response := WebRTCAnswer{
		SDP:  answer.SDP,
		Type: answer.Type.String(),
	}
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(jsonResponse)
}
