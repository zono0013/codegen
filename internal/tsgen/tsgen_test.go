package tsgen

import (
	"strings"
	"testing"

	"github.com/zono0013/codegen/internal/prototest"
)

const testProto = `syntax = "proto3";
package shop.v1;
import "google/protobuf/timestamp.proto";
// @entity
message Item {
  // @pk
  string id = 1;
  // @email
  string contact = 2;
  int32 stock = 3;
  int64 views = 4;
  bool active = 5;
  double rating = 6;
  uint32 rank = 7;
  repeated string tags = 8;
  map<string, int64> counts = 9;
  google.protobuf.Timestamp created_at = 10;
}

// @pk が無いので entity ではない（Handler は unwrap しない）
// @entity
message Draft {
  string title = 1;
}

message EmptyRequest {}
message ListRequest { int32 limit = 1; }
message GetRequest { string id = 1; }
message SearchInRequest {
  string org_id = 1;
  string q = 2;
}
message UpdateRequest {
  string id = 1;
  string contact = 2;
}
message NestedPathRequest {
  string org_id = 1;
  string id = 2;
}

message ListResponse { repeated Item items = 1; }
message GetResponse { Item item = 1; }
message EmptyResponse {}
message StatsResponse {
  Item item = 1;
  int64 total = 2;
  repeated google.protobuf.Timestamp history = 3;
  map<string, google.protobuf.Timestamp> last_seen = 4;
}
message DraftResponse { Draft draft = 1; }
message ByKeyResponse { map<string, Item> items = 1; }

service ItemService {
  // @http GET /api/items/all
  rpc ListAll(EmptyRequest) returns (ListResponse);
  // @http GET /api/items
  rpc List(ListRequest) returns (ListResponse);
  // @http POST /api/items
  rpc Create(UpdateRequest) returns (GetResponse);
  // @http GET /api/items/{id}
  rpc Get(GetRequest) returns (GetResponse);
  // @http GET /api/orgs/{org_id}/items
  rpc SearchIn(SearchInRequest) returns (ListResponse);
  // @http PATCH /api/items/{id}
  rpc Update(UpdateRequest) returns (GetResponse);
  // @http DELETE /api/orgs/{org_id}/items/{id}
  rpc DeleteNested(NestedPathRequest) returns (EmptyResponse);
  // @http GET /api/items/stats
  rpc Stats(EmptyRequest) returns (StatsResponse);
  // @http GET /api/drafts/latest
  rpc LatestDraft(EmptyRequest) returns (DraftResponse);
  // @http GET /api/items/by-key
  rpc ByKey(EmptyRequest) returns (ByKeyResponse);
  // @http が無い rpc は API 関数を作らない
  rpc Internal(EmptyRequest) returns (EmptyResponse);
}
`

func generateFiles(t *testing.T) map[string]string {
	t.Helper()
	return prototest.Generate(t, map[string]string{"shop/v1/shop.proto": testProto}, New(Params{Client: "~/lib/http"}))
}

func assertContains(t *testing.T, file, content, want string) {
	t.Helper()
	if !strings.Contains(content, want) {
		t.Errorf("%s に次の内容が無い:\n%s\n--- 生成結果 ---\n%s", file, want, content)
	}
}

func TestGenerate_APIFunction(t *testing.T) {
	api := generateFiles(t)["api/shop.gen.ts"]
	tests := []struct {
		name string
		want string
	}{
		{
			"request が空なら引数なし",
			"export const listAll = (): Promise<ListResponse> =>\n" +
				"  apiClient.get<ListResponse>('/api/items/all').then(r => r.data)",
		},
		{
			"GET で path param なしなら query string",
			"export const list = (params: ListRequest): Promise<ListResponse> =>\n" +
				"  apiClient.get<ListResponse>('/api/items', { params }).then(r => r.data)",
		},
		{
			"POST で path param なしなら body",
			"export const create = (data: UpdateRequest): Promise<GetResponse> =>\n" +
				"  apiClient.post<GetResponse>('/api/items', data).then(r => r.data)",
		},
		{
			"path param だけなら分割代入してテンプレートリテラル",
			"export const get = ({ id }: GetRequest): Promise<GetResponse> =>\n" +
				"  apiClient.get<GetResponse>(`/api/items/${id}`).then(r => r.data)",
		},
		{
			"GET で path param + 残りフィールドなら残りを query string",
			"export const searchIn = ({ org_id, ...rest }: SearchInRequest): Promise<ListResponse> =>\n" +
				"  apiClient.get<ListResponse>(`/api/orgs/${org_id}/items`, { params: rest }).then(r => r.data)",
		},
		{
			"PATCH で path param + 残りフィールドなら残りを body",
			"export const update = ({ id, ...data }: UpdateRequest): Promise<GetResponse> =>\n" +
				"  apiClient.patch<GetResponse>(`/api/items/${id}`, data).then(r => r.data)",
		},
		{
			"path param が複数",
			"export const deleteNested = ({ org_id, id }: NestedPathRequest): Promise<EmptyResponse> =>\n" +
				"  apiClient.delete<EmptyResponse>(`/api/orgs/${org_id}/items/${id}`).then(r => r.data)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertContains(t, "api/shop.gen.ts", api, tt.want)
		})
	}

	t.Run("@http の無い rpc は関数を作らない", func(t *testing.T) {
		if strings.Contains(api, "export const internal") {
			t.Errorf("Internal の関数が生成されている:\n%s", api)
		}
	})
	t.Run("空 request の型は import しない", func(t *testing.T) {
		for _, line := range strings.Split(api, "\n") {
			if strings.HasPrefix(line, "import type") && strings.Contains(line, "EmptyRequest") {
				t.Errorf("import = %s", line)
			}
		}
	})
}

func TestGenerate_ZodType(t *testing.T) {
	schema := generateFiles(t)["schemas/shop.gen.ts"]
	tests := []struct {
		name string
		want string
	}{
		{"string", "  id: z.string(),"},
		{"@email 付き string", "  contact: z.string().email(),"},
		{"int32 は整数", "  stock: z.number().int(),"},
		{"int64", "  views: z.number(),"},
		{"bool", "  active: z.boolean(),"},
		{"double", "  rating: z.number(),"},
		{"uint32 も整数", "  rank: z.number().int(),"},
		{"repeated は配列", "  tags: z.array(z.string()),"},
		{"map は record", "  counts: z.record(z.string(), z.number()),"},
		{"message 参照はスキーマ参照", "  item: ItemSchema,"},
		// encoding/json は time.Time を RFC 3339 で出す（DTO は UTC の Z、Input / Output はオフセットを持つことがある）
		{"Timestamp は日時の文字列", "  created_at: z.iso.datetime({ offset: true }),"},
		{"repeated Timestamp", "  history: z.array(z.iso.datetime({ offset: true })),"},
		{"map の値の Timestamp", "  last_seen: z.record(z.string(), z.iso.datetime({ offset: true })),"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertContains(t, "schemas/shop.gen.ts", schema, tt.want)
		})
	}
}

func TestGenerate_TimestampHasNoSchema(t *testing.T) {
	// google/protobuf は生成しないので、TimestampSchema を参照・import しない
	schema := generateFiles(t)["schemas/shop.gen.ts"]
	if strings.Contains(schema, "TimestampSchema") || strings.Contains(schema, "google") {
		t.Errorf("Timestamp のスキーマを参照している:\n%s", schema)
	}
}

// Handler は Response message をそのまま返さず中身を返すので、FE の型もそれに合わせる。
func TestGenerate_ResponseShape(t *testing.T) {
	schema := generateFiles(t)["schemas/shop.gen.ts"]
	tests := []struct {
		name string
		want string
	}{
		{"フィールド 0 個は void", "export const EmptyResponseSchema = z.void()"},
		{"entity 1 個はその entity", "export const GetResponseSchema = ItemSchema"},
		{"repeated entity 1 個は配列", "export const ListResponseSchema = z.array(ItemSchema)"},
		{"複数フィールドはオブジェクトのまま", "export const StatsResponseSchema = z.object({"},
		{"@pk の無い @entity は unwrap しない", "export const DraftResponseSchema = z.object({"},
		{"entity の map は unwrap しない", "export const ByKeyResponseSchema = z.object({"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertContains(t, "schemas/shop.gen.ts", schema, tt.want)
		})
	}
}

func TestGenerate_ImportsClientFromParams(t *testing.T) {
	files := generateFiles(t)
	assertContains(t, "api/shop.gen.ts", files["api/shop.gen.ts"], "import { apiClient } from '~/lib/http'")
}

func TestNew_RequiresClient(t *testing.T) {
	_, err := New(Params{})(prototest.Files(t, map[string]string{"shop/v1/shop.proto": testProto}))
	if err == nil || !strings.Contains(err.Error(), "Client") {
		t.Errorf("err = %v, want Client が空であることを示すエラー", err)
	}
}

func TestGenerate_HiddenFieldNotInSchema(t *testing.T) {
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
`}, New(Params{Client: "~/lib/http"}))
	src := files["schemas/user.gen.ts"]
	if strings.Contains(src, "password") {
		t.Errorf("Zod スキーマに @hidden の password が出ている:\n%s", src)
	}
	if !strings.Contains(src, "email:") {
		t.Errorf("@hidden 以外のフィールドまで消えた:\n%s", src)
	}
}
