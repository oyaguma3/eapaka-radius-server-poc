// Package milenage はMilenage計算のラッパーを提供する。
package milenage

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"

	"github.com/wmnsk/milenage"
)

// Vector は認証ベクターを表す。
type Vector struct {
	RAND []byte // 16 bytes
	AUTN []byte // 16 bytes
	XRES []byte // 4-16 bytes（通常8 bytes）
	CK   []byte // 16 bytes
	IK   []byte // 16 bytes
}

// Calculator はMilenage計算を行う。
type Calculator struct{}

// NewCalculator は新しいCalculatorを生成する。
func NewCalculator() *Calculator {
	return &Calculator{}
}

// GenerateVector は認証ベクターを生成する。
func (c *Calculator) GenerateVector(ki, opc, amf []byte, sqn uint64) (*Vector, error) {
	// 1. RAND生成（128bit乱数）
	randVal := make([]byte, 16)
	if _, err := rand.Read(randVal); err != nil {
		return nil, fmt.Errorf("failed to generate RAND: %w", err)
	}

	return c.GenerateVectorWithRAND(ki, opc, amf, sqn, randVal)
}

// GenerateVectorWithRAND は指定されたRANDで認証ベクターを生成する。
// テスト用に公開。
func (c *Calculator) GenerateVectorWithRAND(ki, opc, amf []byte, sqn uint64, randVal []byte) (*Vector, error) {
	// AMFをuint16に変換
	amfVal := binary.BigEndian.Uint16(amf)

	// Milenage構造体を作成
	m := milenage.NewWithOPc(ki, opc, randVal, sqn, amfVal)

	// f2345計算（RES, CK, IK, AK）
	res, ck, ik, ak, err := m.F2345()
	if err != nil {
		return nil, fmt.Errorf("failed to compute f2345: %w", err)
	}

	// f1計算（MAC-A）
	macA, err := m.F1()
	if err != nil {
		return nil, fmt.Errorf("failed to compute f1: %w", err)
	}

	// SQNをバイト列に変換
	sqnBytes := SQNToBytes(sqn)

	// AUTN = (SQN ⊕ AK) || AMF || MAC-A
	autn := c.computeAUTN(sqnBytes, ak, amf, macA)

	return &Vector{
		RAND: randVal,
		AUTN: autn,
		XRES: res,
		CK:   ck,
		IK:   ik,
	}, nil
}

// computeAUTN はAUTNを計算する。
// AUTN = (SQN ⊕ AK) || AMF || MAC-A
func (c *Calculator) computeAUTN(sqn, ak, amf, macA []byte) []byte {
	autn := make([]byte, 16)

	// SQN ⊕ AK (6 bytes)
	for i := 0; i < 6; i++ {
		autn[i] = sqn[i] ^ ak[i]
	}

	// AMF (2 bytes)
	copy(autn[6:8], amf)

	// MAC-A (8 bytes)
	copy(autn[8:16], macA)

	return autn
}

// SQNToBytes はSQN（uint64）を6バイトのバイト列に変換する。
func SQNToBytes(sqn uint64) []byte {
	b := make([]byte, 6)
	b[0] = byte(sqn >> 40)
	b[1] = byte(sqn >> 32)
	b[2] = byte(sqn >> 24)
	b[3] = byte(sqn >> 16)
	b[4] = byte(sqn >> 8)
	b[5] = byte(sqn)
	return b
}

// BytesToSQN は6バイトのバイト列をSQN（uint64）に変換する。
func BytesToSQN(b []byte) uint64 {
	return uint64(b[0])<<40 | uint64(b[1])<<32 | uint64(b[2])<<24 |
		uint64(b[3])<<16 | uint64(b[4])<<8 | uint64(b[5])
}
