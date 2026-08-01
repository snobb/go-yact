package proto

type Hello struct {
	SrcAddr string `json:"source"`
	DstAddr string `json:"target"`
}
