// Code generated by protoc-gen-go. DO NOT EDIT.
// source: chisel.proto

package chprotobuf

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type PbEndpointRole int32

const (
	PbEndpointRole_UNKNOWN  PbEndpointRole = 0
	PbEndpointRole_STUB     PbEndpointRole = 1
	PbEndpointRole_SKELETON PbEndpointRole = 2
)

var PbEndpointRole_name = map[int32]string{
	0: "UNKNOWN",
	1: "STUB",
	2: "SKELETON",
}

var PbEndpointRole_value = map[string]int32{
	"UNKNOWN":  0,
	"STUB":     1,
	"SKELETON": 2,
}

func (x PbEndpointRole) String() string {
	return proto.EnumName(PbEndpointRole_name, int32(x))
}

func (PbEndpointRole) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_166ce0f0cfe77f00, []int{0}
}

type PbEndpointDescriptor struct {
	Role                 PbEndpointRole `protobuf:"varint,1,opt,name=Role,json=role,proto3,enum=PbEndpointRole" json:"Role,omitempty"`
	Type                 string         `protobuf:"bytes,2,opt,name=Type,json=type,proto3" json:"Type,omitempty"`
	Path                 string         `protobuf:"bytes,3,opt,name=Path,json=path,proto3" json:"Path,omitempty"`
	XXX_NoUnkeyedLiteral struct{}       `json:"-"`
	XXX_unrecognized     []byte         `json:"-"`
	XXX_sizecache        int32          `json:"-"`
}

func (m *PbEndpointDescriptor) Reset()         { *m = PbEndpointDescriptor{} }
func (m *PbEndpointDescriptor) String() string { return proto.CompactTextString(m) }
func (*PbEndpointDescriptor) ProtoMessage()    {}
func (*PbEndpointDescriptor) Descriptor() ([]byte, []int) {
	return fileDescriptor_166ce0f0cfe77f00, []int{0}
}

func (m *PbEndpointDescriptor) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PbEndpointDescriptor.Unmarshal(m, b)
}
func (m *PbEndpointDescriptor) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PbEndpointDescriptor.Marshal(b, m, deterministic)
}
func (m *PbEndpointDescriptor) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PbEndpointDescriptor.Merge(m, src)
}
func (m *PbEndpointDescriptor) XXX_Size() int {
	return xxx_messageInfo_PbEndpointDescriptor.Size(m)
}
func (m *PbEndpointDescriptor) XXX_DiscardUnknown() {
	xxx_messageInfo_PbEndpointDescriptor.DiscardUnknown(m)
}

var xxx_messageInfo_PbEndpointDescriptor proto.InternalMessageInfo

func (m *PbEndpointDescriptor) GetRole() PbEndpointRole {
	if m != nil {
		return m.Role
	}
	return PbEndpointRole_UNKNOWN
}

func (m *PbEndpointDescriptor) GetType() string {
	if m != nil {
		return m.Type
	}
	return ""
}

func (m *PbEndpointDescriptor) GetPath() string {
	if m != nil {
		return m.Path
	}
	return ""
}

type PbChannelDescriptor struct {
	Reverse              bool                  `protobuf:"varint,1,opt,name=Reverse,json=reverse,proto3" json:"Reverse,omitempty"`
	StubDescriptor       *PbEndpointDescriptor `protobuf:"bytes,2,opt,name=StubDescriptor,json=stubDescriptor,proto3" json:"StubDescriptor,omitempty"`
	SkeletonDescriptor   *PbEndpointDescriptor `protobuf:"bytes,3,opt,name=SkeletonDescriptor,json=skeletonDescriptor,proto3" json:"SkeletonDescriptor,omitempty"`
	XXX_NoUnkeyedLiteral struct{}              `json:"-"`
	XXX_unrecognized     []byte                `json:"-"`
	XXX_sizecache        int32                 `json:"-"`
}

func (m *PbChannelDescriptor) Reset()         { *m = PbChannelDescriptor{} }
func (m *PbChannelDescriptor) String() string { return proto.CompactTextString(m) }
func (*PbChannelDescriptor) ProtoMessage()    {}
func (*PbChannelDescriptor) Descriptor() ([]byte, []int) {
	return fileDescriptor_166ce0f0cfe77f00, []int{1}
}

func (m *PbChannelDescriptor) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PbChannelDescriptor.Unmarshal(m, b)
}
func (m *PbChannelDescriptor) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PbChannelDescriptor.Marshal(b, m, deterministic)
}
func (m *PbChannelDescriptor) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PbChannelDescriptor.Merge(m, src)
}
func (m *PbChannelDescriptor) XXX_Size() int {
	return xxx_messageInfo_PbChannelDescriptor.Size(m)
}
func (m *PbChannelDescriptor) XXX_DiscardUnknown() {
	xxx_messageInfo_PbChannelDescriptor.DiscardUnknown(m)
}

var xxx_messageInfo_PbChannelDescriptor proto.InternalMessageInfo

func (m *PbChannelDescriptor) GetReverse() bool {
	if m != nil {
		return m.Reverse
	}
	return false
}

func (m *PbChannelDescriptor) GetStubDescriptor() *PbEndpointDescriptor {
	if m != nil {
		return m.StubDescriptor
	}
	return nil
}

func (m *PbChannelDescriptor) GetSkeletonDescriptor() *PbEndpointDescriptor {
	if m != nil {
		return m.SkeletonDescriptor
	}
	return nil
}

type PbSessionConfigRequest struct {
	ClientVersion        string                 `protobuf:"bytes,1,opt,name=ClientVersion,json=clientVersion,proto3" json:"ClientVersion,omitempty"`
	ChannelDescriptors   []*PbChannelDescriptor `protobuf:"bytes,2,rep,name=ChannelDescriptors,json=channelDescriptors,proto3" json:"ChannelDescriptors,omitempty"`
	XXX_NoUnkeyedLiteral struct{}               `json:"-"`
	XXX_unrecognized     []byte                 `json:"-"`
	XXX_sizecache        int32                  `json:"-"`
}

func (m *PbSessionConfigRequest) Reset()         { *m = PbSessionConfigRequest{} }
func (m *PbSessionConfigRequest) String() string { return proto.CompactTextString(m) }
func (*PbSessionConfigRequest) ProtoMessage()    {}
func (*PbSessionConfigRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_166ce0f0cfe77f00, []int{2}
}

func (m *PbSessionConfigRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PbSessionConfigRequest.Unmarshal(m, b)
}
func (m *PbSessionConfigRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PbSessionConfigRequest.Marshal(b, m, deterministic)
}
func (m *PbSessionConfigRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PbSessionConfigRequest.Merge(m, src)
}
func (m *PbSessionConfigRequest) XXX_Size() int {
	return xxx_messageInfo_PbSessionConfigRequest.Size(m)
}
func (m *PbSessionConfigRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_PbSessionConfigRequest.DiscardUnknown(m)
}

var xxx_messageInfo_PbSessionConfigRequest proto.InternalMessageInfo

func (m *PbSessionConfigRequest) GetClientVersion() string {
	if m != nil {
		return m.ClientVersion
	}
	return ""
}

func (m *PbSessionConfigRequest) GetChannelDescriptors() []*PbChannelDescriptor {
	if m != nil {
		return m.ChannelDescriptors
	}
	return nil
}

type PbDialRequest struct {
	UseDescriptor          bool                  `protobuf:"varint,1,opt,name=UseDescriptor,json=useDescriptor,proto3" json:"UseDescriptor,omitempty"`
	ChannelDescriptorIndex int32                 `protobuf:"varint,2,opt,name=ChannelDescriptorIndex,json=channelDescriptorIndex,proto3" json:"ChannelDescriptorIndex,omitempty"`
	SkeletonDescriptor     *PbEndpointDescriptor `protobuf:"bytes,3,opt,name=SkeletonDescriptor,json=skeletonDescriptor,proto3" json:"SkeletonDescriptor,omitempty"`
	StubName               string                `protobuf:"bytes,4,opt,name=StubName,json=stubName,proto3" json:"StubName,omitempty"`
	XXX_NoUnkeyedLiteral   struct{}              `json:"-"`
	XXX_unrecognized       []byte                `json:"-"`
	XXX_sizecache          int32                 `json:"-"`
}

func (m *PbDialRequest) Reset()         { *m = PbDialRequest{} }
func (m *PbDialRequest) String() string { return proto.CompactTextString(m) }
func (*PbDialRequest) ProtoMessage()    {}
func (*PbDialRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_166ce0f0cfe77f00, []int{3}
}

func (m *PbDialRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PbDialRequest.Unmarshal(m, b)
}
func (m *PbDialRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PbDialRequest.Marshal(b, m, deterministic)
}
func (m *PbDialRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PbDialRequest.Merge(m, src)
}
func (m *PbDialRequest) XXX_Size() int {
	return xxx_messageInfo_PbDialRequest.Size(m)
}
func (m *PbDialRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_PbDialRequest.DiscardUnknown(m)
}

var xxx_messageInfo_PbDialRequest proto.InternalMessageInfo

func (m *PbDialRequest) GetUseDescriptor() bool {
	if m != nil {
		return m.UseDescriptor
	}
	return false
}

func (m *PbDialRequest) GetChannelDescriptorIndex() int32 {
	if m != nil {
		return m.ChannelDescriptorIndex
	}
	return 0
}

func (m *PbDialRequest) GetSkeletonDescriptor() *PbEndpointDescriptor {
	if m != nil {
		return m.SkeletonDescriptor
	}
	return nil
}

func (m *PbDialRequest) GetStubName() string {
	if m != nil {
		return m.StubName
	}
	return ""
}

func init() {
	proto.RegisterEnum("PbEndpointRole", PbEndpointRole_name, PbEndpointRole_value)
	proto.RegisterType((*PbEndpointDescriptor)(nil), "PbEndpointDescriptor")
	proto.RegisterType((*PbChannelDescriptor)(nil), "PbChannelDescriptor")
	proto.RegisterType((*PbSessionConfigRequest)(nil), "PbSessionConfigRequest")
	proto.RegisterType((*PbDialRequest)(nil), "PbDialRequest")
}

func init() { proto.RegisterFile("chisel.proto", fileDescriptor_166ce0f0cfe77f00) }

var fileDescriptor_166ce0f0cfe77f00 = []byte{
	// 402 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xac, 0x52, 0xc1, 0x6e, 0xd3, 0x40,
	0x10, 0xc5, 0x89, 0x21, 0xee, 0xa4, 0x09, 0xd1, 0x50, 0x22, 0x8b, 0x53, 0x14, 0x2a, 0x14, 0x71,
	0x70, 0xa4, 0x20, 0xb8, 0x71, 0x69, 0x92, 0x43, 0x55, 0xe4, 0x5a, 0xeb, 0x04, 0x10, 0x37, 0xef,
	0x76, 0x5a, 0xaf, 0x70, 0x77, 0x8d, 0x77, 0x5d, 0xd1, 0x3b, 0xbf, 0xc4, 0x7f, 0xf0, 0x49, 0x28,
	0x36, 0x52, 0x1d, 0x39, 0xe2, 0xd4, 0xdb, 0xce, 0x9b, 0xb7, 0xf3, 0x76, 0xde, 0x3e, 0x38, 0x16,
	0xa9, 0x34, 0x94, 0x05, 0x79, 0xa1, 0xad, 0x9e, 0x0a, 0x38, 0x89, 0xf8, 0x5a, 0x5d, 0xe5, 0x5a,
	0x2a, 0xbb, 0x22, 0x23, 0x0a, 0x99, 0x5b, 0x5d, 0xe0, 0x6b, 0x70, 0x99, 0xce, 0xc8, 0x77, 0x26,
	0xce, 0x6c, 0xb8, 0x78, 0x1e, 0x3c, 0x90, 0x76, 0x30, 0x73, 0x0b, 0x9d, 0x11, 0x22, 0xb8, 0x9b,
	0xfb, 0x9c, 0xfc, 0xce, 0xc4, 0x99, 0x1d, 0x31, 0xd7, 0xde, 0xe7, 0x15, 0x16, 0x25, 0x36, 0xf5,
	0xbb, 0x35, 0x96, 0x27, 0x36, 0x9d, 0xfe, 0x76, 0xe0, 0x45, 0xc4, 0x97, 0x69, 0xa2, 0x14, 0x65,
	0x0d, 0x11, 0x1f, 0x7a, 0x8c, 0xee, 0xa8, 0x30, 0xb5, 0x8e, 0xc7, 0x7a, 0x45, 0x5d, 0xe2, 0x47,
	0x18, 0xc6, 0xb6, 0xe4, 0x0f, 0xdc, 0x4a, 0xa3, 0xbf, 0x78, 0x19, 0x1c, 0x7a, 0x2d, 0x1b, 0x9a,
	0x3d, 0x32, 0xae, 0x01, 0xe3, 0xef, 0x94, 0x91, 0xd5, 0xaa, 0x31, 0xa2, 0xfb, 0xbf, 0x11, 0x68,
	0x5a, 0x17, 0xa6, 0xbf, 0x1c, 0x18, 0x47, 0x3c, 0x26, 0x63, 0xa4, 0x56, 0x4b, 0xad, 0xae, 0xe5,
	0x0d, 0xa3, 0x1f, 0x25, 0x19, 0x8b, 0xa7, 0x30, 0x58, 0x66, 0x92, 0x94, 0xfd, 0x4c, 0xc5, 0xae,
	0x5b, 0x2d, 0x70, 0xc4, 0x06, 0xa2, 0x09, 0xe2, 0x0a, 0xb0, 0xb5, 0xb5, 0xf1, 0x3b, 0x93, 0xee,
	0xac, 0xbf, 0x38, 0x09, 0x0e, 0x58, 0xc2, 0x50, 0xb4, 0xf8, 0xd3, 0x3f, 0x0e, 0x0c, 0x22, 0xbe,
	0x92, 0x49, 0xd6, 0x50, 0xdf, 0x1a, 0x6a, 0xac, 0x56, 0xdb, 0x37, 0x28, 0x9b, 0x20, 0x7e, 0x80,
	0x71, 0x4b, 0xe0, 0x5c, 0x5d, 0xd1, 0xcf, 0xca, 0xcc, 0xa7, 0x6c, 0x2c, 0x0e, 0x76, 0x1f, 0xc9,
	0x3d, 0x7c, 0x05, 0xde, 0xee, 0x0f, 0xc3, 0xe4, 0x96, 0x7c, 0xb7, 0x72, 0xc7, 0x33, 0xff, 0xea,
	0xb7, 0xef, 0x61, 0xb8, 0x9f, 0x28, 0xec, 0x43, 0x6f, 0x1b, 0x5e, 0x84, 0x97, 0x5f, 0xc2, 0xd1,
	0x13, 0xf4, 0xc0, 0x8d, 0x37, 0xdb, 0xb3, 0x91, 0x83, 0xc7, 0xe0, 0xc5, 0x17, 0xeb, 0x4f, 0xeb,
	0xcd, 0x65, 0x38, 0xea, 0x9c, 0xbd, 0xf9, 0x76, 0x7a, 0x23, 0x6d, 0x5a, 0xf2, 0x40, 0xe8, 0xdb,
	0xf9, 0x57, 0xba, 0xd3, 0xe7, 0x4a, 0xcc, 0xeb, 0x40, 0xcf, 0x45, 0x5a, 0x45, 0x9a, 0x97, 0xd7,
	0xfc, 0x59, 0x75, 0x7a, 0xf7, 0x37, 0x00, 0x00, 0xff, 0xff, 0x00, 0x75, 0xb2, 0xd0, 0xec, 0x02,
	0x00, 0x00,
}
