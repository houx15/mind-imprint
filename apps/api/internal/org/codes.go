// Package org holds organization-layer helpers shared by the API handlers:
// distributable code generation for class join codes and teacher invites.
package org

import (
	"crypto/rand"
	"strings"
)

// alphabet excludes visually ambiguous characters (0/O, 1/I/L) so codes are safe
// to print, read aloud, and re-type.
const alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// randString returns n characters drawn uniformly from alphabet using crypto/rand.
func randString(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	var b strings.Builder
	b.Grow(n)
	for _, v := range buf {
		b.WriteByte(alphabet[int(v)%len(alphabet)])
	}
	return b.String(), nil
}

// NewClassJoinCode returns a student-facing class code formatted XXXX-XXXX.
func NewClassJoinCode() (string, error) {
	s, err := randString(8)
	if err != nil {
		return "", err
	}
	return s[:4] + "-" + s[4:], nil
}

// NewTeacherInviteCode returns a teacher invite code prefixed "T-" to keep it
// visually distinct from class join codes.
func NewTeacherInviteCode() (string, error) {
	s, err := randString(8)
	if err != nil {
		return "", err
	}
	return "T-" + s, nil
}
