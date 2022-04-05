package consts

// MaxInsertPacketSizeForVMStorage is the maximum packet size in bytes vmstorage can accept from vmstorage.
// It cannot be reduced due to backwards compatibility :(
const MaxInsertPacketSizeForVMStorage = 100 * 1024 * 1024

// MaxInsertPacketSizeForVMInsert is the maximum packet size in bytes vminsert may send to vmstorage.
// It is smaller than MaxInsertPacketSizeForVMStorage in order to reduce
// max memory usage occupied by buffers at vminsert and vmstorage.
const MaxInsertPacketSizeForVMInsert = 30 * 1024 * 1024
