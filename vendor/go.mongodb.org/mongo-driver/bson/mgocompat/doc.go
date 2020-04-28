// Package mgocompat provides Registry, a BSON registry compatible with globalsign/mgo's BSON,
// with some remaining differences. It also provides RegistryRespectNilValues for compatibility
// with mgo's BSON with RespectNilValues set to true. A registry can be configured on a
// mongo.Client with the SetRegistry option. See the bsoncodec docs for more details on registries.
//
// Registry supports Getter and Setter equivalents by registering hooks. Note that if a value
// matches the hook for bsoncodec.Marshaler, bsoncodec.ValueMarshaler, or bsoncodec.Proxy, that
// hook will take priority over the Getter hook. The same is true for the hooks for
// bsoncodec.Unmarshaler and bsoncodec.ValueUnmarshaler and the Setter hook.
//
// The functional differences between Registry and globalsign/mgo's BSON library are:
//
// 1) Registry errors instead of silently skipping mismatched types when decoding.
//
// 2) Registry does not have special handling for marshaling array ops ("$in", "$nin", "$all").
//
// The driver uses different types than mgo's bson. The differences are:
//
// 1) The driver's bson.RawValue is equivalent to mgo's bson.Raw, but uses Value instead
//    of Data and uses Type, which is a bsontype.Type object that wraps a byte, instead of
//    bson.Raw's Kind, a byte.
//
// 3) The driver uses primitive.ObjectID, which is a [12]byte instead of mgo's
//    bson.ObjectId, a string. Due to this, the zero value marshals and unmarshals differently
//    for Extended JSON, with the driver marshaling as `{"ID":"000000000000000000000000"}` and
//    mgo as `{"Id":""}`. The driver will not unmarshal {"ID":""} to a primitive.ObjectID.
//
// 4) The driver's primitive.Symbol is equivalent to mgo's bson.Symbol.
//
// 5) The driver uses primitive.Timestamp instead of mgo's bson.MongoTimestamp. While
//    MongoTimestamp is an int64, primitive.Timestamp stores the time and counter as two separate
//    uint32 values, T and I respectively.
//
// 6) The driver uses primitive.MinKey and primitive.MaxKey, which are struct{}, instead
//    of mgo's bson.MinKey and bson.MaxKey, which are int64.
//
// 7) The driver's primitive.Undefined is equivalent to mgo's bson.Undefined.
//
// 8) The driver's primitive.Binary is equivalent to mgo's bson.Binary, with variables named Subtype
//    and Data instead of Kind and Data.
//
// 9) The driver's primitive.Regex is equivalent to mgo's bson.RegEx.
//
// 10) The driver's primitive.JavaScript is equivalent to mgo's bson.JavaScript with no
//     scope and primitive.CodeWithScope is equivalent to mgo's bson.JavaScript with scope.
//
// 11) The driver's primitive.DBPointer is equivalent to mgo's bson.DBPointer, with variables
//     named DB and Pointer instead of Namespace and Id.
//
// 12) When implementing the Setter interface, mgocompat.ErrSetZero is equivalent to mgo's
//     bson.ErrSetZero.
//
package mgocompat
