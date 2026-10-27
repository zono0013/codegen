package gogen

import (
	"slices"
	"strings"
	"testing"

	"github.com/zono0013/codegen/internal/prototest"
)

const testProto = `syntax = "proto3";
package shop.v1;
// @entity
message Item {
  // @pk
  string id = 1;
}
message GetItemRequest { string id = 1; }
message GetItemResponse { Item item = 1; }
service ItemService {
  // @http GET /api/items/{id}
  rpc GetItem(GetItemRequest) returns (GetItemResponse);
}
`

func TestGenerate_ImportsUseImportPath(t *testing.T) {
	const importPath = "example.com/shop/backend"
	files := prototest.Generate(t, map[string]string{"shop/v1/shop.proto": testProto}, New(Params{ImportPath: importPath}))

	imported := 0
	for name, src := range files {
		for _, line := range strings.Split(src, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, `"`) || !strings.Contains(line, "/internal/") && !strings.Contains(line, "/pkg/") {
				continue
			}
			imported++
			if !strings.HasPrefix(line, `"`+importPath+"/") {
				t.Errorf("%s: %s が %s 配下を指していない", name, line, importPath)
			}
		}
	}
	if imported == 0 {
		t.Fatal("生成物にプロジェクト内の import が 1 つも無い（テストの前提が崩れている）")
	}
}

func TestNew_RequiresImportPath(t *testing.T) {
	_, err := New(Params{})(prototest.Files(t, map[string]string{"shop/v1/shop.proto": testProto}))
	if err == nil || !strings.Contains(err.Error(), "ImportPath") {
		t.Errorf("err = %v, want ImportPath が空であることを示すエラー", err)
	}
}

func TestGenerate_Runtime(t *testing.T) {
	const importPath = "example.com/shop/backend"
	files := prototest.Generate(t, map[string]string{"shop/v1/shop.proto": testProto}, New(Params{ImportPath: importPath}))

	for _, name := range []string{
		"pkg/util/errs/errors.gen.go",
		"pkg/util/errs/http.gen.go",
		"pkg/util/tx/tx.gen.go",
		"internal/infra/postgres/errors.gen.go",
		"cmd/migration-gen/main.gen.go",
	} {
		if _, ok := files[name]; !ok {
			t.Errorf("%s が生成されていない", name)
		}
	}
	for name, src := range files {
		if strings.Contains(src, importPathPlaceholder) {
			t.Errorf("%s: %s が置き換わっていない", name, importPathPlaceholder)
		}
	}
	if src := files["cmd/migration-gen/main.gen.go"]; !strings.Contains(src, `"`+importPath+`/internal/domain/entity"`) {
		t.Errorf("migration-gen が生成先の entity を import していない:\n%s", src)
	}
}

func TestGenerate_NoEntity(t *testing.T) {
	// migration-gen（常に生成する）が entity.All を参照するので、entity が無くても registry を出す
	files := prototest.Generate(t, map[string]string{"empty/v1/empty.proto": `syntax = "proto3";
package empty.v1;
message Ping {}
`}, New(Params{ImportPath: "example.com/x/server"}))
	if src, ok := files["internal/domain/entity/registry.gen.go"]; !ok || !strings.Contains(src, "var All = []any{") {
		t.Errorf("entity が無いと registry.gen.go が出ない:\n%s", src)
	}
}

func TestGenerate_RuntimeOnlyFromSources(t *testing.T) {
	files := prototest.Generate(t, map[string]string{"shop/v1/shop.proto": testProto}, New(Params{ImportPath: "example.com/x/server"}))
	for name := range files {
		if !strings.HasSuffix(name, ".gen.go") {
			t.Errorf("%s: .gen.go 以外が出力された", name)
		}
	}
}

func TestGenerate_IntegerPK(t *testing.T) {
	// 整数の主キーは DB が採番する（0 は「未採番」）ので、Entity の生成関数で空チェックしない
	files := prototest.Generate(t, map[string]string{"tag/v1/tag.proto": `syntax = "proto3";
package tag.v1;
// @entity
message Tag {
  // @pk
  int64 id = 1;
  // @required
  string name = 2;
}
`}, New(Params{ImportPath: "example.com/x/server"}))
	src := files["internal/domain/entity/tag.gen.go"]
	if strings.Contains(src, `id == ""`) {
		t.Errorf("整数の主キーを文字列として検査している:\n%s", src)
	}
	if !strings.Contains(src, `name == ""`) {
		t.Errorf("@required の検査が消えている:\n%s", src)
	}
}

func TestGenerate_StringPKIsRequired(t *testing.T) {
	files := prototest.Generate(t, map[string]string{"shop/v1/shop.proto": testProto}, New(Params{ImportPath: "example.com/x/server"}))
	if src := files["internal/domain/entity/item.gen.go"]; !strings.Contains(src, `if id == "" {`) {
		t.Errorf("文字列の主キーの空チェックが無い:\n%s", src)
	}
}

const typedParamsProto = `syntax = "proto3";
package tag.v1;
// @entity
message Tag {
  // @pk
  int64 id = 1;
}
message GetTagRequest {
  int64 id = 1;
  int32 rev = 2;
  uint32 page = 3;
  uint64 offset = 4;
  bool archived = 5;
  string q = 6;
}
message GetTagResponse { Tag tag = 1; }
message DeleteTagRequest {
  uint64 id = 1;
  bool force = 2;
}
message DeleteTagResponse {}
service TagService {
  // @http GET /api/tags/{id}/revisions/{rev}
  rpc GetTag(GetTagRequest) returns (GetTagResponse);
  // @http DELETE /api/tags/{id}/{force}
  rpc DeleteTag(DeleteTagRequest) returns (DeleteTagResponse);
}
`

func TestGenerate_TypedParams(t *testing.T) {
	files := prototest.Generate(t, map[string]string{"tag/v1/tag.proto": typedParamsProto}, New(Params{ImportPath: "example.com/x/server"}))
	src := files["internal/handler/tag_handler.gen.go"]
	for _, want := range []string{
		// path は型に合わせて変換し、読めなければ 400 にする
		`_id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)`,
		`_rev, err := strconv.ParseInt(r.PathValue("rev"), 10, 32)`,
		`errs.NewBadRequestError("invalid path parameter: id")`,
		`_id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)`,
		`_force, err := strconv.ParseBool(r.PathValue("force"))`,
		`int32(_rev),`,
		// query の符号なし整数は同じ型で読む（int32 / int64 のまま代入するとコンパイルできない）
		`_page, _ := strconv.ParseUint(urlQuery.Get("page"), 10, 32)`,
		`uint32(_page),`,
		`_offset, _ := strconv.ParseUint(urlQuery.Get("offset"), 10, 64)`,
		`_archived, _ := strconv.ParseBool(urlQuery.Get("archived"))`,
		`_q := urlQuery.Get("q")`,
		`"strconv"`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("%q が無い:\n%s", want, src)
		}
	}
	if strings.Contains(src, `: r.PathValue(`) {
		t.Errorf("整数の path param を文字列のまま代入している:\n%s", src)
	}
}

func TestGenerate_StringPathParamNeedsNoStrconv(t *testing.T) {
	files := prototest.Generate(t, map[string]string{"shop/v1/shop.proto": testProto}, New(Params{ImportPath: "example.com/x/server"}))
	src := files["internal/handler/item_handler.gen.go"]
	if !strings.Contains(src, `: r.PathValue("id"),`) || strings.Contains(src, `"strconv"`) {
		t.Errorf("文字列の path param はそのまま読み、strconv を import しない:\n%s", src)
	}
}

func TestGenerate_OutputEntityIsDTO(t *testing.T) {
	// Output の中の entity も DTO にする。entity には json タグが無いので、そのまま JSON にすると
	// キーが Go のフィールド名（ID / Name）になり、OpenAPI・TS のスキーマ（snake_case）と食い違う
	files := prototest.Generate(t, map[string]string{"tag/v1/tag.proto": `syntax = "proto3";
package tag.v1;
// @entity
message Tag {
  // @pk
  int64 id = 1;
  string name = 2;
}
message TagGroup {
  string label = 1;
  Tag top = 2;
  map<string, Tag> by_name = 3;
}
message GetRandomTagsRequest {}
message GetRandomTagsResponse {
  string message = 1;
  repeated Tag tags = 2;
  TagGroup group = 3;
}
service TagService {
  // @http GET /api/tags/random
  rpc GetRandomTags(GetRandomTagsRequest) returns (GetRandomTagsResponse);
}
`}, New(Params{ImportPath: "example.com/x/server"}))
	src := files["internal/usecase/tag_usecase_interface.gen.go"]
	// テストでは gofmt を通さないので、空白の数は見ない
	flat := strings.Join(strings.Fields(src), " ")
	for _, want := range []string{
		"Tags []*dto.TagDTO `json:\"tags\"`",
		"Top *dto.TagDTO",
		"ByName map[string]*dto.TagDTO",
		`"example.com/x/server/internal/dto"`,
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("%q が無い:\n%s", want, src)
		}
	}
	if strings.Contains(src, "entity.") {
		t.Errorf("Output が entity を参照している:\n%s", src)
	}
}

func TestNormalizeInitialisms(t *testing.T) {
	// Go の慣習（golint / staticcheck の ST1003 の一覧）どおり、頭字語の単語を大文字にする
	tests := map[string]string{
		"Id":            "ID",
		"AuthorId":      "AuthorID",
		"Uuid":          "UUID",
		"CommunityUuid": "CommunityUUID",
		"UserIdList":    "UserIDList",
		"ApiKey":        "APIKey",
		"HttpUrl":       "HTTPURL",
		"RemoteIp":      "RemoteIP",
		"RawSql":        "RawSQL",
		"BodyHtml":      "BodyHTML",
		"Utf8Name":      "UTF8Name",
		// 頭字語ではない単語はそのまま
		"Ipsum":    "Ipsum",
		"Identity": "Identity",
		"Ids":      "Ids",
		"Name":     "Name",
		"XURL":     "XURL",
	}
	for in, want := range tests {
		if got := normalizeInitialisms(in); got != want {
			t.Errorf("normalizeInitialisms(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenerate_TimestampField(t *testing.T) {
	// google.protobuf.Timestamp は Entity / DTO で time.Time。列名と JSON のキーは proto の名前のまま（名前に規則は無い）
	files := prototest.Generate(t, map[string]string{"device/v1/device.proto": `syntax = "proto3";
package device.v1;
import "google/protobuf/timestamp.proto";
// @entity
message Device {
  // @pk
  string device_uuid = 1;
  google.protobuf.Timestamp last_api_call_at = 2;
  google.protobuf.Timestamp created_at = 3;
}
`}, New(Params{ImportPath: "example.com/x/server"}))
	flat := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	ent := flat(files["internal/domain/entity/device.gen.go"])
	for _, want := range []string{
		"DeviceUUID string `gorm:\"column:device_uuid;primaryKey\"`",
		"LastAPICallAt time.Time `gorm:\"column:last_api_call_at;not null\"`",
		"CreatedAt time.Time `gorm:\"column:created_at;not null\"`",
		// 引数名はほかのフィールドと同じく正規化の前の名前から作る（apiKey と同じ形）
		"lastApiCallAt time.Time,",
	} {
		if !strings.Contains(ent, want) {
			t.Errorf("entity に %q が無い:\n%s", want, ent)
		}
	}
	dto := flat(files["internal/dto/device.gen.go"])
	for _, want := range []string{
		"LastAPICallAt time.Time `json:\"last_api_call_at\"`",
		"CreatedAt time.Time `json:\"created_at\"`",
		// DB から読んだ時刻は接続のタイムゾーンを持つので、API では UTC にそろえる
		"CreatedAt: e.CreatedAt.UTC(),",
		`"time"`,
	} {
		if !strings.Contains(dto, want) {
			t.Errorf("DTO に %q が無い:\n%s", want, dto)
		}
	}
	if strings.Contains(dto, "Unix") {
		t.Errorf("DTO が Unix 秒にしている:\n%s", dto)
	}
}

func TestGenerate_TimestampInInputOutput(t *testing.T) {
	// Input / Output の Timestamp も time.Time にする（Seconds / Nanos を持つ struct を作らない）
	files := prototest.Generate(t, map[string]string{"event/v1/event.proto": `syntax = "proto3";
package event.v1;
import "google/protobuf/timestamp.proto";
message Slot {
  google.protobuf.Timestamp start_at = 1;
}
message ReserveRequest {
  google.protobuf.Timestamp start_at = 1;
  repeated google.protobuf.Timestamp candidates = 2;
  map<string, google.protobuf.Timestamp> by_room = 3;
  Slot slot = 4;
}
message ReserveResponse {
  google.protobuf.Timestamp reserved_at = 1;
}
service EventService {
  // @http POST /api/events/reserve
  rpc Reserve(ReserveRequest) returns (ReserveResponse);
}
`}, New(Params{ImportPath: "example.com/x/server"}))
	src := files["internal/usecase/event_usecase_interface.gen.go"]
	flat := strings.Join(strings.Fields(src), " ")
	for _, want := range []string{
		"StartAt time.Time Candidates []time.Time ByRoom map[string]time.Time Slot *Slot",
		"ReservedAt time.Time `json:\"reserved_at\"`",
		"type Slot struct { StartAt time.Time `json:\"start_at\"` }",
		`"time"`,
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("%q が無い:\n%s", want, src)
		}
	}
	if strings.Contains(src, "Timestamp") || strings.Contains(src, "Seconds") {
		t.Errorf("Timestamp の struct を作っている:\n%s", src)
	}
	// Handler は body を自分の struct に読むので、こちらも time を import する
	handler := files["internal/handler/event_handler.gen.go"]
	for _, want := range []string{"StartAt time.Time `json:\"start_at\"`", `"time"`} {
		if !strings.Contains(strings.Join(strings.Fields(handler), " "), want) {
			t.Errorf("Handler に %q が無い:\n%s", want, handler)
		}
	}
}
func TestGenerate_TableNameIsEnglishPlural(t *testing.T) {
	files := prototest.Generate(t, map[string]string{"community/v1/community.proto": `syntax = "proto3";
package community.v1;
// @entity
message Community {
  // @pk
  string uuid = 1;
}
`}, New(Params{ImportPath: "example.com/x/server"}))
	src := files["internal/domain/entity/community.gen.go"]
	if !strings.Contains(src, `return "communities"`) {
		t.Errorf("テーブル名が communities になっていない:\n%s", src)
	}
	if repo := files["internal/domain/repository/community_repository.gen.go"]; !strings.Contains(repo, "communities []*entity.Community") {
		t.Errorf("Bulk 系の引数名が communities になっていない:\n%s", repo)
	}
}

func TestGenerate_TableNameOfKeywordEntity(t *testing.T) {
	// 予約語になる名前（type）でも、テーブル名は複数形、引数名は予約語を避けた形のまま（type_）
	files := prototest.Generate(t, map[string]string{"kind/v1/kind.proto": `syntax = "proto3";
package kind.v1;
// @entity
message Type {
  // @pk
  int64 id = 1;
}
`}, New(Params{ImportPath: "example.com/x/server"}))
	if src := files["internal/domain/entity/type.gen.go"]; !strings.Contains(src, `return "types"`) {
		t.Errorf("テーブル名が types になっていない:\n%s", src)
	}
	if repo := files["internal/domain/repository/type_repository.gen.go"]; !strings.Contains(repo, "type_ []*entity.Type") {
		t.Errorf("Bulk 系の引数名が予約語を避けていない:\n%s", repo)
	}
}

func TestGenerate_HiddenFieldNotInDTO(t *testing.T) {
	// @hidden のフィールドは DB の列（entity）には残し、API に返す DTO からは外す
	files := prototest.Generate(t, map[string]string{"user/v1/user.proto": `syntax = "proto3";
package user.v1;
// @entity
message User {
  // @pk
  string uuid = 1;
  string email = 2;
  // bcrypt のハッシュ
  // @hidden
  bytes password = 3;
}
message GetUserRequest { string uuid = 1; }
message GetUserResponse { User user = 1; }
service UserService {
  // @http GET /api/users/{uuid}
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
}
`}, New(Params{ImportPath: "example.com/x/server"}))
	if ent := files["internal/domain/entity/user.gen.go"]; !strings.Contains(ent, "column:password") {
		t.Errorf("entity から password の列が消えた:\n%s", ent)
	}
	dto := files["internal/dto/user.gen.go"]
	if strings.Contains(dto, "Password") || strings.Contains(dto, "password") {
		t.Errorf("DTO に password が出ている:\n%s", dto)
	}
	if !strings.Contains(dto, "Email") {
		t.Errorf("DTO から @hidden 以外のフィールドまで消えた:\n%s", dto)
	}
}

func TestNew_RejectsMisplacedHidden(t *testing.T) {
	// API として使えなくなる組み合わせ、効かない場所の @hidden は、黙って通さずエラーにする
	for name, src := range map[string]string{
		// 主キーが出ないと、クライアントは GET /x/{id} に渡す値を持てない
		"@pk": `// @entity
message Acct {
  // @pk @hidden
  string id = 1;
}`,
		// cursor の基準が出ないと、次のページを取れない
		"@paging": `// @entity
message Acct {
  // @pk
  string id = 1;
  // @unique @paging @hidden
  int64 seq = 2;
}`,
		// entity ではない message では効かない（出したくないつもりで付けたものが出てしまう）
		"非 entity": `message Plain {
  // @hidden
  string note = 1;
}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := New(Params{ImportPath: "example.com/x/server"})(prototest.Files(t, map[string]string{"a/v1/a.proto": "syntax = \"proto3\";\npackage a.v1;\n" + src + "\n"}))
			if err == nil || !strings.Contains(err.Error(), "@hidden") {
				t.Errorf("err = %v, want @hidden についてのエラー", err)
			}
		})
	}
}

func TestGenerate_EntityScalarTypes(t *testing.T) {
	// Entity のフィールドは Input / Output と同じ Go の型にする（以前は対応外の型が黙って string になっていた）
	files := prototest.Generate(t, map[string]string{"metric/v1/metric.proto": `syntax = "proto3";
package metric.v1;
// @entity
message Metric {
  // @pk
  int64 id = 1;
  sint32 a = 2;
  sfixed64 b = 3;
  uint32 c = 4;
  fixed64 d = 5;
  float e = 6;
  double f = 7;
  bytes g = 8;
  repeated uint32 h = 9;
}
`}, New(Params{ImportPath: "example.com/x/server"}))
	flat := strings.Join(strings.Fields(files["internal/domain/entity/metric.gen.go"]), " ")
	for _, want := range []string{
		"A int32 `", "B int64 `", "C uint32 `", "D uint64 `", "E float32 `", "F float64 `", "G []byte `", "H []uint32 `",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("%q が無い:\n%s", want, flat)
		}
	}
}

func TestNew_RejectsUnsupportedEntityFieldTypes(t *testing.T) {
	// 対応していない型は、黙って string にせずエラーにする
	for name, field := range map[string]string{
		"enum":    "Kind kind = 2;",
		"message": "Meta meta = 2;",
	} {
		t.Run(name, func(t *testing.T) {
			src := `syntax = "proto3";
package metric.v1;
enum Kind { KIND_UNSPECIFIED = 0; }
message Meta { string note = 1; }
// @entity
message Metric {
  // @pk
  int64 id = 1;
  ` + field + `
}
`
			_, err := New(Params{ImportPath: "example.com/x/server"})(prototest.Files(t, map[string]string{"metric/v1/metric.proto": src}))
			if err == nil || !strings.Contains(err.Error(), "Metric") || !strings.Contains(err.Error(), "metric.v1.") {
				t.Errorf("err = %v, want 対応していない型のエラー（どの message のどのフィールドか）", err)
			}
		})
	}
}

func TestNew_RejectsStringValidationOnNonString(t *testing.T) {
	// @email / @required は文字列の検査なので、ほかの型に付けると生成したコードがコンパイルできない
	for _, marker := range []string{"@email", "@required"} {
		t.Run(marker, func(t *testing.T) {
			src := "syntax = \"proto3\";\npackage a.v1;\n// @entity\nmessage Acct {\n  // @pk\n  string id = 1;\n  // " + marker + "\n  bytes mail = 2;\n}\n"
			_, err := New(Params{ImportPath: "example.com/x/server"})(prototest.Files(t, map[string]string{"a/v1/a.proto": src}))
			if err == nil || !strings.Contains(err.Error(), marker) {
				t.Errorf("err = %v, want %s は string だけというエラー", err, marker)
			}
		})
	}
}

func TestGenerate_NilBytesBecomesEmpty(t *testing.T) {
	// nil の []byte は NULL として送られ、NOT NULL の列に入らない
	files := prototest.Generate(t, map[string]string{"a/v1/a.proto": `syntax = "proto3";
package a.v1;
// @entity
message Blob {
  // @pk
  int64 id = 1;
  bytes data = 2;
}
`}, New(Params{ImportPath: "example.com/x/server"}))
	if src := strings.Join(strings.Fields(files["internal/domain/entity/blob.gen.go"]), " "); !strings.Contains(src, "if data == nil { data = []byte{} }") {
		t.Errorf("nil の []byte を空にしていない:\n%s", src)
	}
}

func TestNew_RejectsEntityInRequest(t *testing.T) {
	// request の entity は、主キーや @hidden のフィールドまで外から書けてしまう（一括代入）。
	// request は scalar を並べて書く
	for name, req := range map[string]string{
		"直下":          "message SaveRequest { Item item = 1; }",
		"repeated":    "message SaveRequest { repeated Item items = 1; }",
		"map の値":      "message SaveRequest { map<string, Item> items = 1; }",
		"非 entity の中": "message Wrap { Item item = 1; }\nmessage SaveRequest { Wrap wrap = 1; }",
		"相互再帰の中":      "message A { B b = 1; }\nmessage B { A a = 1; Item item = 2; }\nmessage SaveRequest { A a = 1; }",
	} {
		t.Run(name, func(t *testing.T) {
			src := `syntax = "proto3";
package shop.v1;
// @entity
message Item {
  // @pk
  string id = 1;
}
` + req + `
message SaveResponse {}
service ItemService {
  // @http POST /api/items
  rpc Save(SaveRequest) returns (SaveResponse);
}
`
			_, err := New(Params{ImportPath: "example.com/x/server"})(prototest.Files(t, map[string]string{"shop/v1/shop.proto": src}))
			if err == nil || !strings.Contains(err.Error(), "Save") || !strings.Contains(err.Error(), "Item") {
				t.Errorf("err = %v, want request の entity を指すエラー（rpc と entity の名前）", err)
			}
		})
	}
}

func TestNew_RejectsEntityAsRPCMessage(t *testing.T) {
	// request そのものが entity だと一括代入、response そのものが entity だと @hidden が DTO を通らずに出る
	for name, rpc := range map[string]string{
		"request":  "rpc Save(Item) returns (SaveResponse);",
		"response": "rpc Get(GetRequest) returns (Item);",
	} {
		t.Run(name, func(t *testing.T) {
			src := `syntax = "proto3";
package shop.v1;
// @entity
message Item {
  // @pk
  string id = 1;
}
message SaveResponse {}
message GetRequest { string id = 1; }
service ItemService {
  // @http POST /api/items
  ` + rpc + `
}
`
			_, err := New(Params{ImportPath: "example.com/x/server"})(prototest.Files(t, map[string]string{"shop/v1/shop.proto": src}))
			if err == nil || !strings.Contains(err.Error(), "Item") {
				t.Errorf("err = %v, want entity をそのまま rpc の message に使うエラー", err)
			}
		})
	}
}

func TestNew_RejectsTimestampAnnotation(t *testing.T) {
	// 日時は型（google.protobuf.Timestamp）で表す。注釈は効かないので、黙って int64 のままにしない
	for name, src := range map[string]string{
		"entity":    "syntax = \"proto3\";\npackage a.v1;\n// @entity\nmessage Post {\n  // @pk\n  int64 id = 1;\n  // @timestamp\n  int64 created_at_unix = 2;\n}\n",
		"entity 以外": "syntax = \"proto3\";\npackage a.v1;\nmessage Post {\n  // @timestamp\n  int64 created_at_unix = 1;\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := New(Params{ImportPath: "example.com/x/server"})(prototest.Files(t, map[string]string{"a/v1/a.proto": src}))
			if err == nil || !strings.Contains(err.Error(), "google.protobuf.Timestamp") {
				t.Errorf("err = %v, want Timestamp 型を使うよう案内するエラー", err)
			}
		})
	}
}
func TestNew_RejectsEntityOnlyMarkersOutsideEntity(t *testing.T) {
	// entity ではない message（入れ子も）では効かないので、黙って通さない
	for _, marker := range []string{"@required", "@unique", "@decimal(10,2)", "@hidden"} {
		t.Run(marker, func(t *testing.T) {
			src := "syntax = \"proto3\";\npackage a.v1;\nmessage Outer {\n  message Inner {\n    // " + marker + "\n    string x = 1;\n  }\n  Inner inner = 1;\n}\n"
			_, err := New(Params{ImportPath: "example.com/x/server"})(prototest.Files(t, map[string]string{"a/v1/a.proto": src}))
			want := strings.SplitN(marker, "(", 2)[0]
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("err = %v, want %s は entity だけというエラー", err, want)
			}
		})
	}
}

func TestNew_RejectsTimestampKey(t *testing.T) {
	// time.Time を引数に取る SelectBy / SelectByCursor は生成していない。timestamptz の配列にも対応していない
	for name, field := range map[string]string{
		"@unique":  "// @unique\n  google.protobuf.Timestamp at = 2;",
		"repeated": "repeated google.protobuf.Timestamp at = 2;",
	} {
		t.Run(name, func(t *testing.T) {
			src := "syntax = \"proto3\";\npackage a.v1;\nimport \"google/protobuf/timestamp.proto\";\n// @entity\nmessage Post {\n  // @pk\n  int64 id = 1;\n  " + field + "\n}\n"
			_, err := New(Params{ImportPath: "example.com/x/server"})(prototest.Files(t, map[string]string{"a/v1/a.proto": src}))
			if err == nil || !strings.Contains(err.Error(), "google.protobuf.Timestamp") {
				t.Errorf("err = %v, want Timestamp と %s の併用のエラー", err, name)
			}
		})
	}
}
func TestGenerate_SnakeNames(t *testing.T) {
	// テーブル名・ファイル名は message 名から（頭字語は 1 語）、列名・JSON のキーは proto のフィールド名そのまま
	files := prototest.Generate(t, map[string]string{"log/v1/log.proto": `syntax = "proto3";
package log.v1;
import "google/protobuf/timestamp.proto";
// @entity
message HTTPLog {
  // @pk
  int64 id = 1;
  int32 pos_x_y = 2;
  google.protobuf.Timestamp created_at = 3;
}
`}, New(Params{ImportPath: "example.com/x/server"}))
	src, ok := files["internal/domain/entity/http_log.gen.go"]
	if !ok {
		t.Fatalf("entity のファイル名が http_log.gen.go になっていない: %v", keys(files))
	}
	flat := strings.Join(strings.Fields(src), " ")
	for _, want := range []string{`return "http_logs"`, "column:pos_x_y;", "column:created_at;"} {
		if !strings.Contains(flat, want) {
			t.Errorf("%q が無い:\n%s", want, src)
		}
	}
	dto := strings.Join(strings.Fields(files["internal/dto/http_log.gen.go"]), " ")
	for _, want := range []string{"`json:\"pos_x_y\"`", "`json:\"created_at\"`"} {
		if !strings.Contains(dto, want) {
			t.Errorf("DTO に %q が無い:\n%s", want, dto)
		}
	}
}

func TestNew_RejectsNonSnakeEntityField(t *testing.T) {
	// 列名・JSON のキーは proto のフィールド名をそのまま使うので、lower_snake_case に限る
	src := "syntax = \"proto3\";\npackage a.v1;\n// @entity\nmessage Post {\n  // @pk\n  int64 id = 1;\n  string authorName = 2;\n}\n"
	_, err := New(Params{ImportPath: "example.com/x/server"})(prototest.Files(t, map[string]string{"a/v1/a.proto": src}))
	if err == nil || !strings.Contains(err.Error(), "authorName") {
		t.Errorf("err = %v, want lower_snake_case でないフィールドのエラー", err)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
