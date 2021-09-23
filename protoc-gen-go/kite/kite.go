package kite

import (
	"fmt"
	"strconv"
	"strings"

	pb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/generator"
)

// generatedCodeVersion indicates a version of the generated code.
// It is incremented whenever an incompatibility between the generated code and
// the k.Pc package is introduced; the generated code references
// a constant, k.Pc.SupportPackageIsVersionN (where N is generatedCodeVersion).
const generatedCodeVersion = 6

// Paths for packages used by code generated in this file,
// relative to the import_prefix of the generator.Generator.
const (
	contextPkgPath = "context"
	kitePkgPath    = "git.dhgames.cn/svr_comm/kiteg"
	codePkgPath    = "git.dhgames.cn/svr_comm/kiteg/codes"
	statusPkgPath  = "git.dhgames.cn/svr_comm/kiteg/status"
)

func init() {
	generator.RegisterPlugin(new(kite))
}

type kite struct {
	gen      *generator.Generator
	services []string
}

// The names for packages imported in the generated code.
// They may vary from the final path component of the import path
// if the name is used by other packages.
var (
	contextPkg string
	kitePkg    string
)

// Given a type name defined in a .proto, return its object.
// Also record that we're using it, to guarantee the associated import.
func (k *kite) objectNamed(name string) generator.Object {
	k.gen.RecordTypeUse(name)
	return k.gen.ObjectNamed(name)
}

// Given a type name defined in a .proto, return its name as we will print it.
func (k *kite) typeName(str string) string {
	return k.gen.TypeName(k.objectNamed(str))
}

// P forwards to k.gen.P.
func (k *kite) P(args ...interface{}) { k.gen.P(args...) }

func (k *kite) Name() string {
	return "kite"
}

// Init initializes the plugin.
func (k *kite) Init(gen *generator.Generator) {
	k.gen = gen
}

func (k *kite) Generate(file *generator.FileDescriptor) {
	if len(file.FileDescriptorProto.Service) == 0 {
		return
	}

	contextPkg = string(k.gen.AddImport(contextPkgPath))
	kitePkg = string(k.gen.AddImport(kitePkgPath))

	k.P("// Reference imports to suppress errors if they are not otherwise used.")
	k.P("var _ ", contextPkg, ".Context")
	k.P("var _ ", kitePkg, ".ClientConnInterface")
	k.P()

	// Assert version compatibility.
	k.P("// This is a compile-time assertion to ensure that this generated file")
	k.P("// is compatible with the k.Pc package it is being compiled against.")
	k.P("const _ = ", kitePkg, ".SupportPackageIsVersion", generatedCodeVersion)
	k.P()

	for i, service := range file.FileDescriptorProto.Service {
		k.generateService(file, service, i)
	}
}

func (k *kite) GenerateImports(file *generator.FileDescriptor) {
}

// reservedClientName records whether a client name is reserved on the client side.
var reservedClientName = map[string]bool{
	// TODO: do we need any in k.PC?
}

func unexport(s string) string { return strings.ToLower(s[:1]) + s[1:] }

// deprecationComment is the standard comment added to deprecated
// messages, fields, enums, and enum values.
var deprecationComment = "// Deprecated: Do not use."

// generateService generates all the code for the named service.
func (k *kite) generateService(file *generator.FileDescriptor, service *pb.ServiceDescriptorProto, index int) {
	path := fmt.Sprintf("6,%d", index) // 6 means service.

	origServName := service.GetName()
	fullServName := origServName
	if pkg := file.GetPackage(); pkg != "" {
		fullServName = pkg + "." + fullServName
	}
	servName := generator.CamelCase(origServName)
	deprecated := service.GetOptions().GetDeprecated()

	k.P()
	k.P(fmt.Sprintf(`// %sClient is the client API for %s service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.`, servName, servName))

	// Client interface.
	if deprecated {
		k.P("//")
		k.P(deprecationComment)
	}
	k.P("type ", servName, "Client interface {")
	for i, method := range service.Method {
		k.gen.PrintComments(fmt.Sprintf("%s,2,%d", path, i)) // 2 means method in a service.
		if method.GetOptions().GetDeprecated() {
			k.P("//")
			k.P(deprecationComment)
		}
		k.P(k.generateClientSignature(servName, method))
	}
	k.P("}")
	k.P()

	// Client structure.
	k.P("type ", unexport(servName), "Client struct {")
	k.P("cc ", kitePkg, ".ClientConnInterface")
	k.P("}")
	k.P()

	// NewClient factory.
	if deprecated {
		k.P(deprecationComment)
	}
	k.P("func New", servName, "Client (cc ", kitePkg, ".ClientConnInterface) ", servName, "Client {")
	k.P("return &", unexport(servName), "Client{cc}")
	k.P("}")
	k.P()

	var methodIndex, streamIndex int
	serviceDescVar := "_" + servName + "_serviceDesc"
	// Client method implementations.
	for _, method := range service.Method {
		var descExpr string
		if !method.GetServerStreaming() && !method.GetClientStreaming() {
			// Unary RPC method
			descExpr = fmt.Sprintf("&%s.Methods[%d]", serviceDescVar, methodIndex)
			methodIndex++
		} else {
			// Streaming RPC method
			descExpr = fmt.Sprintf("&%s.Streams[%d]", serviceDescVar, streamIndex)
			streamIndex++
		}
		k.generateClientMethod(servName, fullServName, serviceDescVar, method, descExpr)
	}

	// Server interface.
	serverType := servName + "Server"
	k.P("// ", serverType, " is the server API for ", servName, " service.")
	if deprecated {
		k.P("//")
		k.P(deprecationComment)
	}
	k.P("type ", serverType, " interface {")
	for i, method := range service.Method {
		k.gen.PrintComments(fmt.Sprintf("%s,2,%d", path, i)) // 2 means method in a service.
		if method.GetOptions().GetDeprecated() {
			k.P("//")
			k.P(deprecationComment)
		}
		k.P(k.generateServerSignature(servName, method))
	}
	k.P("}")
	k.P()

	// Server Unimplemented struct for forward compatibility.
	if deprecated {
		k.P(deprecationComment)
	}
	k.generateUnimplementedServer(servName, service)

	// Server registration.
	if deprecated {
		k.P(deprecationComment)
	}
	k.P("func Register", servName, "Server(s *", kitePkg, ".Server, srv ", serverType, ") {")
	k.P("s.RegisterService(&", serviceDescVar, `, srv)`)
	k.P("}")
	k.P()

	// Server handler implementations.
	var handlerNames []string
	for _, method := range service.Method {
		hname := k.generateServerMethod(servName, fullServName, method)
		handlerNames = append(handlerNames, hname)
	}

	// Service descriptor.
	k.P("var ", serviceDescVar, " = ", kitePkg, ".ServiceDesc {")
	k.P("ServiceName: ", strconv.Quote(fullServName), ",")
	k.P("HandlerType: (*", serverType, ")(nil),")
	k.P("Methods: []", kitePkg, ".MethodDesc{")
	for i, method := range service.Method {
		if method.GetServerStreaming() || method.GetClientStreaming() {
			continue
		}
		k.P("{")
		k.P("MethodName: ", strconv.Quote(method.GetName()), ",")
		k.P("Handler: ", handlerNames[i], ",")
		k.P("},")
	}
	k.P("},")
	k.P("Streams: []", kitePkg, ".StreamDesc{")
	for i, method := range service.Method {
		if !method.GetServerStreaming() && !method.GetClientStreaming() {
			continue
		}
		k.P("{")
		k.P("StreamName: ", strconv.Quote(method.GetName()), ",")
		k.P("Handler: ", handlerNames[i], ",")
		if method.GetServerStreaming() {
			k.P("ServerStreams: true,")
		}
		if method.GetClientStreaming() {
			k.P("ClientStreams: true,")
		}
		k.P("},")
	}
	k.P("},")
	k.P("Metadata: \"", file.GetName(), "\",")
	k.P("}")
	k.P()
}

// generateUnimplementedServer creates the unimplemented server struct
func (k *kite) generateUnimplementedServer(servName string, service *pb.ServiceDescriptorProto) {
	serverType := servName + "Server"
	k.P("// Unimplemented", serverType, " can be embedded to have forward compatible implementations.")
	k.P("type Unimplemented", serverType, " struct {")
	k.P("}")
	k.P()
	// Unimplemented<service_name>Server's concrete methods
	for _, method := range service.Method {
		k.generateServerMethodConcrete(servName, method)
	}
	k.P()
}

// generateServerMethodConcrete returns unimplemented methods which ensure forward compatibility
func (k *kite) generateServerMethodConcrete(servName string, method *pb.MethodDescriptorProto) {
	header := k.generateServerSignatureWithParamNames(servName, method)
	k.P("func (*Unimplemented", servName, "Server) ", header, " {")
	var nilArg string
	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		nilArg = "nil, "
	}
	methName := generator.CamelCase(method.GetName())
	statusPkg := string(k.gen.AddImport(statusPkgPath))
	codePkg := string(k.gen.AddImport(codePkgPath))
	k.P("return ", nilArg, statusPkg, `.Errorf(`, codePkg, `.Unimplemented, "method `, methName, ` not implemented")`)
	k.P("}")
}

// generateClientSignature returns the client-side signature for a method.
func (k *kite) generateClientSignature(servName string, method *pb.MethodDescriptorProto) string {
	origMethName := method.GetName()
	methName := generator.CamelCase(origMethName)
	if reservedClientName[methName] {
		methName += "_"
	}
	reqArg := ", in *" + k.typeName(method.GetInputType())
	if method.GetClientStreaming() {
		reqArg = ""
	}
	respName := "*" + k.typeName(method.GetOutputType())
	if method.GetServerStreaming() || method.GetClientStreaming() {
		respName = servName + "_" + generator.CamelCase(origMethName) + "Client"
	}
	return fmt.Sprintf("%s(ctx %s.Context%s, opts ...%s.CallOption) (%s, error)", methName, contextPkg, reqArg, kitePkg, respName)
}

func (k *kite) generateClientMethod(servName, fullServName, serviceDescVar string, method *pb.MethodDescriptorProto, descExpr string) {
	sname := fmt.Sprintf("/%s/%s", fullServName, method.GetName())
	methName := generator.CamelCase(method.GetName())
	inType := k.typeName(method.GetInputType())
	outType := k.typeName(method.GetOutputType())

	if method.GetOptions().GetDeprecated() {
		k.P(deprecationComment)
	}
	k.P("func (c *", unexport(servName), "Client) ", k.generateClientSignature(servName, method), "{")
	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		k.P("out := new(", outType, ")")
		// TODO: Pass descExpr to Invoke.
		k.P(`err := c.cc.Invoke(ctx, "`, sname, `", in, out, opts...)`)
		k.P("if err != nil { return nil, err }")
		k.P("return out, nil")
		k.P("}")
		k.P()
		return
	}
	streamType := unexport(servName) + methName + "Client"
	k.P("stream, err := c.cc.NewStream(ctx, ", descExpr, `, "`, sname, `", opts...)`)
	k.P("if err != nil { return nil, err }")
	k.P("x := &", streamType, "{stream}")
	if !method.GetClientStreaming() {
		k.P("if err := x.ClientStream.SendMsg(in); err != nil { return nil, err }")
		k.P("if err := x.ClientStream.CloseSend(); err != nil { return nil, err }")
	}
	k.P("return x, nil")
	k.P("}")
	k.P()

	genSend := method.GetClientStreaming()
	genRecv := method.GetServerStreaming()
	genCloseAndRecv := !method.GetServerStreaming()

	// Stream auxiliary types and methods.
	k.P("type ", servName, "_", methName, "Client interface {")
	if genSend {
		k.P("Send(*", inType, ") error")
	}
	if genRecv {
		k.P("Recv() (*", outType, ", error)")
	}
	if genCloseAndRecv {
		k.P("CloseAndRecv() (*", outType, ", error)")
	}
	k.P(kitePkg, ".ClientStream")
	k.P("}")
	k.P()

	k.P("type ", streamType, " struct {")
	k.P(kitePkg, ".ClientStream")
	k.P("}")
	k.P()

	if genSend {
		k.P("func (x *", streamType, ") Send(m *", inType, ") error {")
		k.P("return x.ClientStream.SendMsg(m)")
		k.P("}")
		k.P()
	}
	if genRecv {
		k.P("func (x *", streamType, ") Recv() (*", outType, ", error) {")
		k.P("m := new(", outType, ")")
		k.P("if err := x.ClientStream.RecvMsg(m); err != nil { return nil, err }")
		k.P("return m, nil")
		k.P("}")
		k.P()
	}
	if genCloseAndRecv {
		k.P("func (x *", streamType, ") CloseAndRecv() (*", outType, ", error) {")
		k.P("if err := x.ClientStream.CloseSend(); err != nil { return nil, err }")
		k.P("m := new(", outType, ")")
		k.P("if err := x.ClientStream.RecvMsg(m); err != nil { return nil, err }")
		k.P("return m, nil")
		k.P("}")
		k.P()
	}
}

// generateServerSignatureWithParamNames returns the server-side signature for a method with parameter names.
func (k *kite) generateServerSignatureWithParamNames(servName string, method *pb.MethodDescriptorProto) string {
	origMethName := method.GetName()
	methName := generator.CamelCase(origMethName)
	if reservedClientName[methName] {
		methName += "_"
	}

	var reqArgs []string
	ret := "error"
	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		reqArgs = append(reqArgs, "ctx "+contextPkg+".Context")
		ret = "(*" + k.typeName(method.GetOutputType()) + ", error)"
	}
	if !method.GetClientStreaming() {
		reqArgs = append(reqArgs, "req *"+k.typeName(method.GetInputType()))
	}
	if method.GetServerStreaming() || method.GetClientStreaming() {
		reqArgs = append(reqArgs, "srv "+servName+"_"+generator.CamelCase(origMethName)+"Server")
	}

	return methName + "(" + strings.Join(reqArgs, ", ") + ") " + ret
}

// generateServerSignature returns the server-side signature for a method.
func (k *kite) generateServerSignature(servName string, method *pb.MethodDescriptorProto) string {
	origMethName := method.GetName()
	methName := generator.CamelCase(origMethName)
	if reservedClientName[methName] {
		methName += "_"
	}

	var reqArgs []string
	ret := "error"
	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		reqArgs = append(reqArgs, contextPkg+".Context")
		ret = "(*" + k.typeName(method.GetOutputType()) + ", error)"
	}
	if !method.GetClientStreaming() {
		reqArgs = append(reqArgs, "*"+k.typeName(method.GetInputType()))
	}
	if method.GetServerStreaming() || method.GetClientStreaming() {
		reqArgs = append(reqArgs, servName+"_"+generator.CamelCase(origMethName)+"Server")
	}

	return methName + "(" + strings.Join(reqArgs, ", ") + ") " + ret
}

func (k *kite) generateServerMethod(servName, fullServName string, method *pb.MethodDescriptorProto) string {
	methName := generator.CamelCase(method.GetName())
	hname := fmt.Sprintf("_%s_%s_Handler", servName, methName)
	inType := k.typeName(method.GetInputType())
	outType := k.typeName(method.GetOutputType())

	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		k.P("func ", hname, "(srv interface{}, ctx ", contextPkg, ".Context, dec func(interface{}) error, interceptor ", kitePkg, ".UnaryServerInterceptor) (interface{}, error) {")
		k.P("in := new(", inType, ")")
		k.P("if err := dec(in); err != nil { return nil, err }")
		k.P("if interceptor == nil { return srv.(", servName, "Server).", methName, "(ctx, in) }")
		k.P("info := &", kitePkg, ".UnaryServerInfo{")
		k.P("Server: srv,")
		k.P("FullMethod: ", strconv.Quote(fmt.Sprintf("/%s/%s", fullServName, methName)), ",")
		k.P("}")
		k.P("handler := func(ctx ", contextPkg, ".Context, req interface{}) (interface{}, error) {")
		k.P("return srv.(", servName, "Server).", methName, "(ctx, req.(*", inType, "))")
		k.P("}")
		k.P("return interceptor(ctx, in, info, handler)")
		k.P("}")
		k.P()
		return hname
	}
	streamType := unexport(servName) + methName + "Server"
	k.P("func ", hname, "(srv interface{}, stream ", kitePkg, ".ServerStream) error {")
	if !method.GetClientStreaming() {
		k.P("m := new(", inType, ")")
		k.P("if err := stream.RecvMsg(m); err != nil { return err }")
		k.P("return srv.(", servName, "Server).", methName, "(m, &", streamType, "{stream})")
	} else {
		k.P("return srv.(", servName, "Server).", methName, "(&", streamType, "{stream})")
	}
	k.P("}")
	k.P()

	genSend := method.GetServerStreaming()
	genSendAndClose := !method.GetServerStreaming()
	genRecv := method.GetClientStreaming()

	// Stream auxiliary types and methods.
	k.P("type ", servName, "_", methName, "Server interface {")
	if genSend {
		k.P("Send(*", outType, ") error")
	}
	if genSendAndClose {
		k.P("SendAndClose(*", outType, ") error")
	}
	if genRecv {
		k.P("Recv() (*", inType, ", error)")
	}
	k.P(kitePkg, ".ServerStream")
	k.P("}")
	k.P()

	k.P("type ", streamType, " struct {")
	k.P(kitePkg, ".ServerStream")
	k.P("}")
	k.P()

	if genSend {
		k.P("func (x *", streamType, ") Send(m *", outType, ") error {")
		k.P("return x.ServerStream.SendMsg(m)")
		k.P("}")
		k.P()
	}
	if genSendAndClose {
		k.P("func (x *", streamType, ") SendAndClose(m *", outType, ") error {")
		k.P("return x.ServerStream.SendMsg(m)")
		k.P("}")
		k.P()
	}
	if genRecv {
		k.P("func (x *", streamType, ") Recv() (*", inType, ", error) {")
		k.P("m := new(", inType, ")")
		k.P("if err := x.ServerStream.RecvMsg(m); err != nil { return nil, err }")
		k.P("return m, nil")
		k.P("}")
		k.P()
	}

	return hname
}
