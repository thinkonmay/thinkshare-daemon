package validator



type Pair struct {
	PeerA string `json:"peerA"`
	PeerB string `json:"peerB"`
}


type Validator interface {
	Validate(queue []string) ([]Pair, []string)
}