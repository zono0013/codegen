package gogen

import (
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
// entity を入力に持つ rpc（Handler / Usecase interface が entity を import する分岐を通すため）
message ImportItemsRequest { repeated Item items = 1; }
message ImportItemsResponse {}
service ItemService {
  // @http GET /api/items/{id}
  rpc GetItem(GetItemRequest) returns (GetItemResponse);
  // @http POST /api/items/import
  rpc ImportItems(ImportItemsRequest) returns (ImportItemsResponse);
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

func TestGenerate_TimestampWithInitialism(t *testing.T) {
	// @timestamp の名前の途中に頭字語があっても、列名と DTO の JSON キーは proto の名前のまま（TS・OpenAPI と同じ）
	files := prototest.Generate(t, map[string]string{"device/v1/device.proto": `syntax = "proto3";
package device.v1;
// @entity
message Device {
  // @pk
  string device_uuid = 1;
  // @timestamp
  int64 last_api_call_at_unix = 2;
  // @timestamp
  int64 created_at_unix = 3;
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
	if dto := flat(files["internal/dto/device.gen.go"]); !strings.Contains(dto, "`json:\"last_api_call_at_unix\"`") {
		t.Errorf("DTO の JSON キーが last_api_call_at_unix になっていない:\n%s", dto)
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
