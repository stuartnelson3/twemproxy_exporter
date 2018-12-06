package main

import "encoding/json"

type stats struct {
	Service          string  `json:"service"`
	Source           string  `json:"source"`
	Version          string  `json:"version"`
	Uptime           float64 `json:"uptime"`
	Timestamp        float64 `json:"timestamp"`
	TotalConnections float64 `json:"total_connections"`
	CurrConnections  float64 `json:"curr_connections"`
	Pools            map[string]*pool
}

func (s *stats) UnmarshalJSON(b []byte) error {
	var dat map[string]interface{}
	if err := json.Unmarshal(b, &dat); err != nil {
		return err
	}
	s.Service = dat["service"].(string)
	delete(dat, "service")
	s.Source = dat["source"].(string)
	delete(dat, "source")
	s.Version = dat["version"].(string)
	delete(dat, "version")
	s.Uptime = dat["uptime"].(float64)
	delete(dat, "uptime")
	s.Timestamp = dat["timestamp"].(float64)
	delete(dat, "timestamp")
	s.TotalConnections = dat["total_connections"].(float64)
	delete(dat, "total_connections")
	s.CurrConnections = dat["curr_connections"].(float64)
	delete(dat, "curr_connections")

	pools := make(map[string]*pool, len(dat))
	d, err := json.Marshal(dat)
	if err != nil {
		return err
	}
	err = json.Unmarshal(d, &pools)
	if err != nil {
		return err
	}
	s.Pools = pools

	return nil
}
