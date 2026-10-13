package z2m

// Device is one entry of Zigbee2MQTT's bridge/devices, with the fields zigbeat reads.
type Device struct {
	Name        string      `json:"friendly_name"`
	Type        string      `json:"type"`
	Disabled    bool        `json:"disabled"`
	Supported   bool        `json:"supported"`
	Interview   string      `json:"interview_state"`
	Interviewed bool        `json:"interview_completed"`
	ModelID     string      `json:"model_id"`
	PowerSource string      `json:"power_source"`
	Definition  *Definition `json:"definition"`
}

type Definition struct {
	Source      string   `json:"source"`
	Model       string   `json:"model"`
	Vendor      string   `json:"vendor"`
	Description string   `json:"description"`
	Exposes     []Expose `json:"exposes"`
}

// Expose is one capability of a device.
type Expose struct {
	Type     string   `json:"type"`
	Name     string   `json:"name"`
	Property string   `json:"property"`
	Endpoint string   `json:"endpoint"`
	Access   int      `json:"access"`
	Features []Expose `json:"features"`
}

// Find returns the device called name.
func Find(devs []Device, name string) (Device, bool) {
	for _, d := range devs {
		if d.Name == name {
			return d, true
		}
	}
	return Device{}, false
}
