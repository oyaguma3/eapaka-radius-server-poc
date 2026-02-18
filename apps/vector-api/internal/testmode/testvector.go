// Package testmode はE2Eテスト用の固定ベクター生成を提供する。
package testmode

import (
	"fmt"
	"strings"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/milenage"
)

// 3GPP TS 35.208 テストベクター（セット1）
// これらはE2Eテスト用の固定値
var (
	// テスト用の固定RAND
	testRAND = []byte{
		0x23, 0x55, 0x3c, 0xbe, 0x96, 0x37, 0xa8, 0x9d,
		0x21, 0x8a, 0xe6, 0x4d, 0xae, 0x47, 0xbf, 0x35,
	}

	// テスト用Ki（128bit）
	testKi = []byte{
		0x46, 0x5b, 0x5c, 0xe8, 0xb1, 0x99, 0xb4, 0x9f,
		0xaa, 0x5f, 0x0a, 0x2e, 0xe2, 0x38, 0xa6, 0xbc,
	}

	// テスト用OPc（128bit）
	testOPc = []byte{
		0xcd, 0x63, 0xcb, 0x71, 0x95, 0x4a, 0x9f, 0x4e,
		0x48, 0xa5, 0x99, 0x4e, 0x37, 0xa0, 0x2b, 0xaf,
	}

	// テスト用AMF（16bit）
	testAMF = []byte{0xb9, 0xb9}

	// テスト用SQN
	testSQN uint64 = 0xff9bb4d0b607
)

// TestVectorProvider はテストベクター生成を行う。
type TestVectorProvider struct {
	imsiPrefix string
}

// NewTestVectorProvider は新しいTestVectorProviderを生成する。
func NewTestVectorProvider(imsiPrefix string) *TestVectorProvider {
	return &TestVectorProvider{
		imsiPrefix: imsiPrefix,
	}
}

// IsTestIMSI は指定されたIMSIがテスト対象かどうかを判定する。
func (p *TestVectorProvider) IsTestIMSI(imsi string) bool {
	return strings.HasPrefix(imsi, p.imsiPrefix)
}

// GetTestVector はテスト用の固定ベクターを生成する。
// 3GPP TS 35.208のテストデータを使用。
func (p *TestVectorProvider) GetTestVector(imsi string) (*milenage.Vector, error) {
	if !p.IsTestIMSI(imsi) {
		return nil, fmt.Errorf("IMSI %s is not a test IMSI", imsi)
	}

	calc := milenage.NewCalculator()
	return calc.GenerateVectorWithRAND(testKi, testOPc, testAMF, testSQN, testRAND)
}

// GetTestCryptoParams はテスト用の暗号パラメータを返す（Ki, OPc, AMF）。
// 防御的コピーにより呼び出し元がスライスを変更しても元のテストデータに影響しない。
func (p *TestVectorProvider) GetTestCryptoParams() (ki, opc, amf []byte) {
	ki = make([]byte, len(testKi))
	copy(ki, testKi)
	opc = make([]byte, len(testOPc))
	copy(opc, testOPc)
	amf = make([]byte, len(testAMF))
	copy(amf, testAMF)
	return
}

// GetDefaultSQN はテストベクターのデフォルトSQN値を返す。
func (p *TestVectorProvider) GetDefaultSQN() uint64 {
	return testSQN
}
