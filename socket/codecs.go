package socket

type Codecs interface {
	// 编码消息
	// 将消息结构编码为Packet
	Encode(o interface{}) (Packet, error)

	// 解码消息
	// 将二进制数据解码为消息结构
	Decode(bytes []byte) (interface{}, error)
}
