package config

// ORSConfig holds SHM key and size parameters for ORS communication.
// These correspond to hftbase ShmManager configuration.
type ORSConfig struct {
	MDShmKey          int `yaml:"md_shm_key"`
	MDQueueSize       int `yaml:"md_queue_size"`
	ReqShmKey         int `yaml:"req_shm_key"`
	ReqQueueSize      int `yaml:"req_queue_size"`
	RespShmKey        int `yaml:"resp_shm_key"`
	RespQueueSize     int `yaml:"resp_queue_size"`
	ClientStoreShmKey int `yaml:"client_store_shm_key"`
}
