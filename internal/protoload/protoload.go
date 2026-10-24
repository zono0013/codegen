// Package protoload は proto をコンパイルし、ジェネレータが読む protogen.File を作る。
package protoload

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// FromDir は dir 配下の全 .proto をコンパイルする。
func FromDir(dir string) ([]*protogen.File, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".proto") {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		paths = append(paths, filepath.ToSlash(rel))
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, err)
	}
	return compile(&protocompile.SourceResolver{ImportPaths: []string{dir}}, paths)
}

// FromSource は「パス → proto ソース」をコンパイルする。
func FromSource(files map[string]string) ([]*protogen.File, error) {
	resolver := &protocompile.SourceResolver{
		Accessor: func(path string) (io.ReadCloser, error) {
			src, ok := files[path]
			if !ok {
				return nil, os.ErrNotExist
			}
			return io.NopCloser(strings.NewReader(src)), nil
		},
	}
	return compile(resolver, slices.Collect(maps.Keys(files)))
}

func compile(resolver protocompile.Resolver, paths []string) ([]*protogen.File, error) {
	slices.Sort(paths)
	compiler := protocompile.Compiler{
		Resolver:       protocompile.WithStandardImports(resolver),
		SourceInfoMode: protocompile.SourceInfoStandard, // アノテーションはコメントに書くため
	}
	compiled, err := compiler.Compile(context.Background(), paths...)
	if err != nil {
		return nil, fmt.Errorf("compile proto: %w", err)
	}

	// protogen は依存が先に並んだ ProtoFile を要求する
	var fdps []*descriptorpb.FileDescriptorProto
	seen := map[string]bool{}
	var add func(fd protoreflect.FileDescriptor)
	add = func(fd protoreflect.FileDescriptor) {
		if seen[fd.Path()] {
			return
		}
		seen[fd.Path()] = true
		imports := fd.Imports()
		for i := 0; i < imports.Len(); i++ {
			add(imports.Get(i).FileDescriptor)
		}
		fdps = append(fdps, protodesc.ToFileDescriptorProto(fd))
	}
	for _, f := range compiled {
		add(f)
	}

	// protogen は Go の import path を必須とするが、生成物では使わないので proto に go_package を書かせずに済むよう仮の値を渡す
	var params []string
	for _, p := range paths {
		params = append(params, "M"+p+"=proto/"+path.Dir(p))
	}
	plugin, err := protogen.Options{}.New(&pluginpb.CodeGeneratorRequest{
		FileToGenerate: paths,
		ProtoFile:      fdps,
		Parameter:      proto.String(strings.Join(params, ",")),
	})
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(plugin.Files, func(f *protogen.File) bool { return !f.Generate }), nil
}
