package app

type Status struct {
	Name      string `json:"name"`
	Ready     int    `json:"ready"`
	Replicas  int    `json:"replicas"`
	Available bool   `json:"available"`
}
