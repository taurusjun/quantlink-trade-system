package config

import (
	"os"
	"path/filepath"
	"testing"
)

// ==================== control_file.go ====================

func TestParseControlFile(t *testing.T) {
	// 创建临时 control 文件（C++ 格式: 单行空格分隔）
	content := "ag_F_3_SFE ./models/model.ag2603.ag2605.par.txt.92201 SFE 16 TB_PAIR_STRAT 0900 1500 ag_F_5_SFE\n"
	path := writeTempFile(t, "control.*.txt", content)

	cc, err := ParseControlFile(path)
	if err != nil {
		t.Fatalf("ParseControlFile: %v", err)
	}

	if cc.BaseName != "ag_F_3_SFE" {
		t.Errorf("BaseName: got %q, want %q", cc.BaseName, "ag_F_3_SFE")
	}
	if cc.ModelFile != "./models/model.ag2603.ag2605.par.txt.92201" {
		t.Errorf("ModelFile: got %q, want %q", cc.ModelFile, "./models/model.ag2603.ag2605.par.txt.92201")
	}
	if cc.Exchange != "SFE" {
		t.Errorf("Exchange: got %q, want %q", cc.Exchange, "SFE")
	}
	if cc.ID != "16" {
		t.Errorf("ID: got %q, want %q", cc.ID, "16")
	}
	if cc.ExecStrat != "TB_PAIR_STRAT" {
		t.Errorf("ExecStrat: got %q, want %q", cc.ExecStrat, "TB_PAIR_STRAT")
	}
	if cc.StartTime != "0900" {
		t.Errorf("StartTime: got %q, want %q", cc.StartTime, "0900")
	}
	if cc.EndTime != "1500" {
		t.Errorf("EndTime: got %q, want %q", cc.EndTime, "1500")
	}
	if cc.SecondName != "ag_F_5_SFE" {
		t.Errorf("SecondName: got %q, want %q", cc.SecondName, "ag_F_5_SFE")
	}
}

func TestParseControlFile_SkipsComments(t *testing.T) {
	content := "# comment line\n\nag_F_3_SFE ./models/m.txt SFE 16 TB_PAIR_STRAT 0900 1500 ag_F_5_SFE\n"
	path := writeTempFile(t, "control.*.txt", content)

	cc, err := ParseControlFile(path)
	if err != nil {
		t.Fatalf("ParseControlFile: %v", err)
	}
	if cc.BaseName != "ag_F_3_SFE" {
		t.Errorf("BaseName: got %q, want %q", cc.BaseName, "ag_F_3_SFE")
	}
}

func TestParseControlFile_TooFewTokens(t *testing.T) {
	content := "ag_F_3_SFE ./models/m.txt SFE\n"
	path := writeTempFile(t, "control.*.txt", content)

	_, err := ParseControlFile(path)
	if err == nil {
		t.Fatal("expected error for too few tokens")
	}
}

func TestParseControlFile_EmptyFile(t *testing.T) {
	path := writeTempFile(t, "control.*.txt", "# only comments\n\n")

	_, err := ParseControlFile(path)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}

// ==================== BaseNameToSymbol ====================

func TestBaseNameToSymbol(t *testing.T) {
	tests := []struct {
		baseName   string
		yearPrefix string
		want       string
	}{
		{"ag_F_3_SFE", "26", "ag2603"},
		{"ag_F_5_SFE", "26", "ag2605"},
		{"au_F_4_SFE", "26", "au2604"},
		{"au_F_6_SFE", "26", "au2606"},
		{"rb_F_10_SFE", "26", "rb2610"},
		{"cu_F_1_SFE", "25", "cu2501"},
		{"al_F_12_SFE", "26", "al2612"},
	}

	for _, tt := range tests {
		got, err := BaseNameToSymbol(tt.baseName, tt.yearPrefix)
		if err != nil {
			t.Errorf("BaseNameToSymbol(%q, %q): %v", tt.baseName, tt.yearPrefix, err)
			continue
		}
		if got != tt.want {
			t.Errorf("BaseNameToSymbol(%q, %q) = %q, want %q", tt.baseName, tt.yearPrefix, got, tt.want)
		}
	}
}

func TestBaseNameToSymbol_InvalidFormat(t *testing.T) {
	_, err := BaseNameToSymbol("ag_O_C_10_1_576_SFE", "26")
	if err == nil {
		t.Fatal("expected error for option format")
	}
}

// ==================== cfg_file.go ====================

func TestParseCfgFile(t *testing.T) {
	content := `LOGGER_THREAD_CPU_AFFINITY =
LOGGER_THREAD_SCHED_POLICY = SCHED_BATCH
INTERACTION_MODE = LIVE
EXCHANGES=CHINA_SHFE
PRODUCT = AG

######## END #############

[CHINA_SHFE]
MDSHMKEY           = 4097
ORSREQUESTSHMKEY   = 8193
ORSRESPONSESHMKEY  = 12289
CLIENTSTORESHMKEY  = 16385
MDSHMSIZE          = 2048
ORSREQUESTSHMSIZE  = 1024
ORSRESPONSESHMSIZE = 1024
`
	path := writeTempFile(t, "config.*.cfg", content)

	cfg, err := ParseCfgFile(path)
	if err != nil {
		t.Fatalf("ParseCfgFile: %v", err)
	}

	if cfg.Product != "AG" {
		t.Errorf("Product: got %q, want %q", cfg.Product, "AG")
	}
	if cfg.Exchanges != "CHINA_SHFE" {
		t.Errorf("Exchanges: got %q, want %q", cfg.Exchanges, "CHINA_SHFE")
	}
	if cfg.GlobalKeys["INTERACTION_MODE"] != "LIVE" {
		t.Errorf("GlobalKeys[INTERACTION_MODE]: got %q", cfg.GlobalKeys["INTERACTION_MODE"])
	}

	// Section 检查
	section, ok := cfg.Sections["CHINA_SHFE"]
	if !ok {
		t.Fatal("section [CHINA_SHFE] not found")
	}
	if section["MDSHMKEY"] != "4097" {
		t.Errorf("MDSHMKEY: got %q, want %q", section["MDSHMKEY"], "4097")
	}
	if section["ORSREQUESTSHMKEY"] != "8193" {
		t.Errorf("ORSREQUESTSHMKEY: got %q, want %q", section["ORSREQUESTSHMKEY"], "8193")
	}
}

func TestCfgConfig_GetExchangeConfig(t *testing.T) {
	content := `EXCHANGES=CHINA_SHFE
PRODUCT = AG

[CHINA_SHFE]
MDSHMKEY           = 4097
ORSREQUESTSHMKEY   = 8193
ORSRESPONSESHMKEY  = 12289
CLIENTSTORESHMKEY  = 16385
MDSHMSIZE          = 2048
ORSREQUESTSHMSIZE  = 1024
ORSRESPONSESHMSIZE = 1024
`
	path := writeTempFile(t, "config.*.cfg", content)

	cfg, err := ParseCfgFile(path)
	if err != nil {
		t.Fatalf("ParseCfgFile: %v", err)
	}

	mdKey, reqKey, respKey, csKey, mdSize, reqSize, respSize, err := cfg.GetExchangeConfig("")
	if err != nil {
		t.Fatalf("GetExchangeConfig: %v", err)
	}

	if mdKey != 4097 {
		t.Errorf("mdKey: got %d, want %d", mdKey, 4097)
	}
	if reqKey != 8193 {
		t.Errorf("reqKey: got %d, want %d", reqKey, 8193)
	}
	if respKey != 12289 {
		t.Errorf("respKey: got %d, want %d", respKey, 12289)
	}
	if csKey != 16385 {
		t.Errorf("clientStoreKey: got %d, want %d", csKey, 16385)
	}
	if mdSize != 2048 {
		t.Errorf("mdSize: got %d, want %d", mdSize, 2048)
	}
	if reqSize != 1024 {
		t.Errorf("reqSize: got %d, want %d", reqSize, 1024)
	}
	if respSize != 1024 {
		t.Errorf("respSize: got %d, want %d", respSize, 1024)
	}
}

// ==================== model_file.go ====================

func TestParseModelFile(t *testing.T) {
	content := `ag_F_3_SFE FUTCOM Dependant ag_F_3_SFE 1 1 1 1 0.5 3
ag_F_5_SFE FUTCOM Independent ag_F_5_SFE 1 1 1 1 0.5 3
# This is a comment
BEGIN_PLACE 0.3
LONG_PLACE 2.0
SHORT_PLACE -2.0
ALPHA 0.01
SIZE 1
MAX_SIZE 10
#DEP_STD_DEV 0.5
#LOOKAHEAD 10
T_VALUE 2.58
`
	path := writeTempFile(t, "model.*.par.txt", content)

	mc, err := ParseModelFile(path)
	if err != nil {
		t.Fatalf("ParseModelFile: %v", err)
	}

	// 检查 indicators
	if len(mc.Indicators) != 2 {
		t.Fatalf("Indicators: got %d, want 2", len(mc.Indicators))
	}
	if mc.Indicators[0].BaseName != "ag_F_3_SFE" {
		t.Errorf("Indicator[0].BaseName: got %q", mc.Indicators[0].BaseName)
	}
	if mc.Indicators[0].Type != "FUTCOM" {
		t.Errorf("Indicator[0].Type: got %q", mc.Indicators[0].Type)
	}
	if mc.Indicators[0].IndName != "Dependant" {
		t.Errorf("Indicator[0].IndName: got %q", mc.Indicators[0].IndName)
	}
	if len(mc.Indicators[0].Args) != 7 {
		t.Errorf("Indicator[0].Args: got %d, want 7", len(mc.Indicators[0].Args))
	}

	// 检查 thresholds
	if mc.Thresholds["BEGIN_PLACE"] != "0.3" {
		t.Errorf("BEGIN_PLACE: got %q", mc.Thresholds["BEGIN_PLACE"])
	}
	if mc.Thresholds["LONG_PLACE"] != "2.0" {
		t.Errorf("LONG_PLACE: got %q", mc.Thresholds["LONG_PLACE"])
	}
	if mc.Thresholds["SHORT_PLACE"] != "-2.0" {
		t.Errorf("SHORT_PLACE: got %q", mc.Thresholds["SHORT_PLACE"])
	}
	if mc.Thresholds["SIZE"] != "1" {
		t.Errorf("SIZE: got %q", mc.Thresholds["SIZE"])
	}
	if mc.Thresholds["MAX_SIZE"] != "10" {
		t.Errorf("MAX_SIZE: got %q", mc.Thresholds["MAX_SIZE"])
	}

	// 检查特殊 #XXX 参数
	if mc.Thresholds["DEP_STD_DEV"] != "0.5" {
		t.Errorf("DEP_STD_DEV: got %q", mc.Thresholds["DEP_STD_DEV"])
	}
	if mc.Thresholds["LOOKAHEAD"] != "10" {
		t.Errorf("LOOKAHEAD: got %q", mc.Thresholds["LOOKAHEAD"])
	}
}

func TestLoadThresholdSet(t *testing.T) {
	mc := &ModelConfig{
		Thresholds: map[string]string{
			"BEGIN_PLACE": "0.3",
			"LONG_PLACE":  "2.0",
			"SHORT_PLACE": "-2.0",
			"MAX_SIZE":    "10",
			"SIZE":        "1",
			"ALPHA":       "0.01",
		},
	}

	ts := LoadThresholdSet(mc)
	if ts.BeginPlace != 0.3 {
		t.Errorf("BeginPlace: got %f, want 0.3", ts.BeginPlace)
	}
	if ts.LongPlace != 2.0 {
		t.Errorf("LongPlace: got %f, want 2.0", ts.LongPlace)
	}
	if ts.ShortPlace != -2.0 {
		t.Errorf("ShortPlace: got %f, want -2.0", ts.ShortPlace)
	}
	if ts.MaxSize != 10 {
		t.Errorf("MaxSize: got %d, want 10", ts.MaxSize)
	}
	if ts.Size != 1 {
		t.Errorf("Size: got %d, want 1", ts.Size)
	}
	if ts.Alpha != 0.01 {
		t.Errorf("Alpha: got %f, want 0.01", ts.Alpha)
	}
}

func TestUpperToSnake(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"BEGIN_PLACE", "begin_place"},
		{"MAX_SIZE", "max_size"},
		{"ALPHA", "alpha"},
		{"SHORT_PLACE", "short_place"},
	}
	for _, tt := range tests {
		got := UpperToSnake(tt.in)
		if got != tt.want {
			t.Errorf("UpperToSnake(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ==================== build_config.go helpers ====================

func TestControlFileExchangeToName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"SFE", "SHFE"},
		{"ZCE", "ZCE"},
		{"CZCE", "ZCE"},
		{"DCE", "DCE"},
		{"CFFEX", "CFFEX"},
		{"GFEX", "GFEX"},
		{"UNKNOWN", "UNKNOWN"},
	}
	for _, tt := range tests {
		got := controlFileExchangeToName(tt.in)
		if got != tt.want {
			t.Errorf("controlFileExchangeToName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExtractProduct(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"ag2603", "ag"},
		{"au2604", "au"},
		{"rb2505", "rb"},
		{"sc2612", "sc"},
		{"al2501", "al"},
	}
	for _, tt := range tests {
		got := extractProduct(tt.in)
		if got != tt.want {
			t.Errorf("extractProduct(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestBuildDefaultInstrumentConfig(t *testing.T) {
	// ag → tickSize=1.0, lotSize=15
	cfg := buildDefaultInstrumentConfig("SHFE", "ag2603")
	if cfg.TickSize != 1.0 {
		t.Errorf("ag TickSize: got %f, want 1.0", cfg.TickSize)
	}
	if cfg.LotSize != 15 {
		t.Errorf("ag LotSize: got %f, want 15", cfg.LotSize)
	}
	if cfg.Exchange != "SHFE" {
		t.Errorf("Exchange: got %q, want %q", cfg.Exchange, "SHFE")
	}

	// au → tickSize=0.02, lotSize=1000
	cfg = buildDefaultInstrumentConfig("SHFE", "au2604")
	if cfg.TickSize != 0.02 {
		t.Errorf("au TickSize: got %f, want 0.02", cfg.TickSize)
	}
	if cfg.LotSize != 1000 {
		t.Errorf("au LotSize: got %f, want 1000", cfg.LotSize)
	}
}

// ==================== 辅助函数 ====================

func writeTempFile(t *testing.T, pattern, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, pattern)
	// 使用 pattern 的第一部分作为文件名
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	_ = path
	return f.Name()
}
