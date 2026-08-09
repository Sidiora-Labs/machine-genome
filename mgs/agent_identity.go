package mgs

import (
	"encoding/hex"
	"errors"
	"strings"
)

// AgentIdentity is the public identity derived from an agent's secured record.
// The Agent ID and surname cannot be selected or transferred independently of
// the Gene, while the given name is an explicit controller-signed claim.
type AgentIdentity struct {
	ID         string `json:"id"`
	GivenName  string `json:"givenName"`
	FamilyName string `json:"familyName"`
	Name       string `json:"name"`
}

// DeriveAgentIdentity returns the canonical human and machine identity for an
// agent Gene. Model, dataset, harness, and class records do not receive one.
func DeriveAgentIdentity(record *GenesisRecord, gene string) (AgentIdentity, error) {
	if record == nil || (record.Identity.SubjectType != "agent" && record.Identity.SubjectType != "agent-instance") {
		return AgentIdentity{}, errors.New("agent identity is only defined for agent and agent-instance records")
	}
	if record.Identity.AgentName == nil || !validGivenName(record.Identity.AgentName.Given) {
		return AgentIdentity{}, errors.New("agent identity requires a valid signed given name claim")
	}
	surname, err := GenomeSurname(gene)
	if err != nil {
		return AgentIdentity{}, err
	}
	given := strings.TrimSpace(record.Identity.AgentName.Given)
	return AgentIdentity{
		ID: "urn:mgs:agent:" + gene, GivenName: given, FamilyName: surname,
		Name: given + " " + surname,
	}, nil
}

// GenomeSurname deterministically produces a readable, collision-resistant
// family name from the 256-bit Gene digest. The 64-bit suffix remains covered
// by the registry's uniqueness constraint; the Agent ID retains all 256 bits.
func GenomeSurname(gene string) (string, error) {
	if err := ValidateDigest(gene); err != nil {
		return "", err
	}
	decoded, _ := decodeMultibase(gene)
	digest := decoded[2:]
	prefixes := [...]string{"Ar", "Bel", "Cor", "Dar", "El", "Fen", "Gal", "Hal", "Is", "Jun", "Kel", "Lor", "Mor", "Nor", "Or", "Per"}
	middles := [...]string{"a", "e", "i", "o", "u", "ae", "ia", "io", "or", "en", "al", "ir", "on", "ul", "er", "an"}
	suffixes := [...]string{"den", "far", "len", "mar", "nor", "ran", "sen", "tor", "vale", "wen", "xen", "yor", "zen", "riel", "voss", "mere"}
	word := prefixes[digest[0]>>4] + middles[digest[1]>>4] + suffixes[digest[2]>>4]
	return word + "-" + hex.EncodeToString(digest[3:11]), nil
}
