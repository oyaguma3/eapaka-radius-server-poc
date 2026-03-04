package radius

// AccountingAttributes はAccounting-Requestから抽出された属性を表す
type AccountingAttributes struct {
	AcctStatusType  uint32   // Acct-Status-Type（1:Start, 2:Stop, 3:Interim, 7:On, 8:Off）
	AcctSessionID   string   // Acct-Session-Id（Start/Stop/Interimでは必須、On/Offではオプション）
	ClassUUID       string   // Class属性からパースしたUUID（空文字列の場合あり）
	UserName        string   // User-Name（オプション）
	NasIdentifier   string   // NAS-Identifier（オプション）
	NasIPAddress    string   // NAS-IP-Address
	FramedIPAddress string   // Framed-IP-Address
	InputOctets     uint32   // Acct-Input-Octets
	OutputOctets    uint32   // Acct-Output-Octets
	SessionTime     uint32   // Acct-Session-Time
	ProxyStates     [][]byte // Proxy-State属性（複数可）
}

// Acct-Status-Type値（RFC 2866）
const (
	AcctStatusTypeStart   uint32 = 1
	AcctStatusTypeStop    uint32 = 2
	AcctStatusTypeInterim uint32 = 3
	AcctStatusTypeOn      uint32 = 7
	AcctStatusTypeOff     uint32 = 8
)
