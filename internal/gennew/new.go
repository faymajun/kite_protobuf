package gennew

import "google.golang.org/protobuf/compiler/protogen"

func GenNew(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile) {
	for _, message := range file.Messages {
		if len(message.GoIdent.GoName) > 3 && message.GoIdent.GoName[0:3] == "Req" {
			g.P("func (m *", message.GoIdent, ") New() *", message.GoIdent, "{ return &", message.GoIdent, "{} }")
		}
	}
}
