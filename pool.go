package main

import "encoding/json"

type pool struct {
	ClientEOF         float64 `json:"client_eof"`
	ClientErr         float64 `json:"client_err"`
	ClientConnections float64 `json:"client_connections"`
	ServerEjects      float64 `json:"server_ejects"`
	ForwardError      float64 `json:"forward_error"`
	Fragments         float64 `json:"fragments"`
	Servers           map[string]*server
}

func (p *pool) UnmarshalJSON(b []byte) error {
	var dat map[string]interface{}
	if err := json.Unmarshal(b, &dat); err != nil {
		return err
	}
	p.ClientEOF = dat["client_eof"].(float64)
	delete(dat, "client_eof")
	p.ClientErr = dat["client_err"].(float64)
	delete(dat, "client_err")
	p.ClientConnections = dat["client_connections"].(float64)
	delete(dat, "client_connections")
	p.ServerEjects = dat["server_ejects"].(float64)
	delete(dat, "server_ejects")
	p.ForwardError = dat["forward_error"].(float64)
	delete(dat, "forward_error")
	p.Fragments = dat["fragments"].(float64)
	delete(dat, "fragments")

	servers := make(map[string]*server, len(dat))
	d, err := json.Marshal(dat)
	if err != nil {
		return err
	}
	err = json.Unmarshal(d, &servers)
	if err != nil {
		return err
	}
	p.Servers = servers

	return nil
}

type server struct {
	ServerEOF         float64 `json:"server_eof"`
	ServerErr         float64 `json:"server_err"`
	ServerTimedout    float64 `json:"server_timedout"`
	ServerConnections float64 `json:"server_connections"`
	ServerEjectedAt   float64 `json:"server_ejected_at"`
	Requests          float64 `json:"requests"`
	RequestBytes      float64 `json:"request_bytes"`
	Responses         float64 `json:"responses"`
	ResponseBytes     float64 `json:"response_bytes"`
	InQueue           float64 `json:"in_queue"`
	InQueueBytes      float64 `json:"in_queue_bytes"`
	OutQueue          float64 `json:"out_queue"`
	OutQueueBytes     float64 `json:"out_queue_bytes"`
}
