package cli

import (
	"encoding/binary"
	"fmt"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

const (
	virtualScriptPointerSize       = 8
	maxDisasmDepth                 = 256
	maxDisasmWarnings              = 256
	maxFieldPathSegments           = 1024
	inlineInstrumentationEventType = uint8(4)
)

type serializedDisasmDecoder struct {
	data         []byte
	names        []uasset.NameEntry
	asset        *uasset.Asset
	instructions []disasmInstruction
	warnings     []string
	truncated    bool
	failedAt     int
}

func disassembleBytecodeWithAsset(data []byte, names []uasset.NameEntry, asset *uasset.Asset) disasmResult {
	decoder := &serializedDisasmDecoder{
		data:     data,
		names:    names,
		asset:    asset,
		failedAt: -1,
	}
	return decoder.decode()
}

func (d *serializedDisasmDecoder) decode() disasmResult {
	if len(d.data) == 0 {
		return disasmResult{DecodeFailedAt: -1}
	}

	ser := 0
	vm := 0
	guard := len(d.data) * 8
	if guard < 64 {
		guard = 64
	}
	for ser < len(d.data) && guard > 0 {
		nextSer, nextVM, token, ok := d.parseExpr(ser, vm, 0)
		if !ok {
			d.truncated = true
			d.failedAt = ser
			break
		}
		if nextSer <= ser {
			d.warnf("decode stalled at serialized offset 0x%04X", ser)
			d.truncated = true
			d.failedAt = ser
			break
		}
		ser, vm = nextSer, nextVM
		guard--
		if token == 0x53 && ser == len(d.data) {
			break
		}
	}
	if guard == 0 && !d.truncated {
		d.warnf("decode guard exhausted")
		d.truncated = true
		d.failedAt = ser
	}

	endFound := false
	for _, inst := range d.instructions {
		if inst.Token == 0x53 {
			endFound = true
			break
		}
	}
	if !endFound {
		d.warnf("EX_EndOfScript not found in selected range")
	}
	if len(d.data) > 0 && d.data[len(d.data)-1] != 0x53 {
		d.warnf("selected range does not end with EX_EndOfScript")
	}

	return disasmResult{
		Instructions:   d.instructions,
		Warnings:       d.warnings,
		Truncated:      d.truncated,
		DecodeFailedAt: d.failedAt,
		VMSize:         vm,
	}
}

func (d *serializedDisasmDecoder) parseExpr(ser, vm, depth int) (int, int, uint8, bool) {
	if ser < 0 || ser >= len(d.data) {
		return ser, vm, 0, false
	}
	if depth > maxDisasmDepth {
		d.warnf("max decode depth exceeded at 0x%04X", ser)
		return ser, vm, 0, false
	}

	token := d.data[ser]
	inst := disasmInstruction{
		Offset:   ser,
		VMOffset: vm,
		Token:    token,
		Opcode:   kismetOpcodeName(token),
	}
	d.instructions = append(d.instructions, inst)
	instIndex := len(d.instructions) - 1

	ser++
	vm++

	setParam := func(k string, v any) {
		if d.instructions[instIndex].Params == nil {
			d.instructions[instIndex].Params = map[string]any{}
		}
		d.instructions[instIndex].Params[k] = v
	}

	switch token {
	case 0x00, 0x01, 0x02, 0x33, 0x48, 0x6C: // property pointers
		fieldPath, ok := d.readFieldPath(&ser, &vm)
		if !ok {
			d.warnf("truncated %s field path at 0x%04X", inst.Opcode, inst.Offset)
			return ser, vm, token, false
		}
		setParam("fieldPath", fieldPath)

	case 0x1F: // EX_StringConst
		val, next, ok := readNullTerminatedString(d.data, ser)
		if !ok {
			d.warnf("truncated EX_StringConst at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("value", val)
		vm += next - ser
		ser = next

	case 0x34: // EX_UnicodeStringConst
		val, next, ok := readNullTerminatedUTF16String(d.data, ser)
		if !ok {
			d.warnf("truncated EX_UnicodeStringConst at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("value", val)
		vm += next - ser
		ser = next

	case 0x1D: // EX_IntConst
		v, ok := d.readInt32(&ser, &vm, 4)
		if !ok {
			d.warnf("truncated EX_IntConst at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("value", v)

	case 0x35: // EX_Int64Const
		v, ok := d.readInt64(&ser, &vm, 8)
		if !ok {
			d.warnf("truncated EX_Int64Const at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("value", v)

	case 0x36: // EX_UInt64Const
		v, ok := d.readUint64(&ser, &vm, 8)
		if !ok {
			d.warnf("truncated EX_UInt64Const at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("value", v)

	case 0x1E: // EX_FloatConst
		bits, ok := d.readUint32(&ser, &vm, 4)
		if !ok {
			d.warnf("truncated EX_FloatConst at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("bits", bits)

	case 0x37: // EX_DoubleConst
		bits, ok := d.readUint64(&ser, &vm, 8)
		if !ok {
			d.warnf("truncated EX_DoubleConst at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("bits", bits)

	case 0x24: // EX_ByteConst
		v, ok := d.readUint8(&ser, &vm, 1)
		if !ok {
			d.warnf("truncated EX_ByteConst at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("value", v)

	case 0x2C: // EX_IntConstByte
		v, ok := d.readUint8(&ser, &vm, 1)
		if !ok {
			d.warnf("truncated EX_IntConstByte at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("value", int8(v))

	case 0x20: // EX_ObjectConst
		idx, ok := d.readObjectPointer(&ser, &vm)
		if !ok {
			d.warnf("truncated EX_ObjectConst at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("index", idx)
		if resolved := d.resolvePackageIndex(idx); resolved != "" {
			setParam("resolved", resolved)
		}

	case 0x21: // EX_NameConst
		ref, ok := d.readScriptName(&ser, &vm)
		if !ok {
			d.warnf("truncated EX_NameConst at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("index", ref.Index)
		setParam("number", ref.Number)
		setParam("name", ref.Display(d.names))

	case 0x06: // EX_Jump
		target, ok := d.readUint32(&ser, &vm, 4)
		if !ok {
			d.warnf("truncated EX_Jump at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("targetVmOffset", int(target))

	case 0x07: // EX_JumpIfNot
		target, ok := d.readUint32(&ser, &vm, 4)
		if !ok {
			d.warnf("truncated EX_JumpIfNot at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("targetVmOffset", int(target))
		setParam("conditionVmOffset", vm)
		nextSer, nextVM, _, childOK := d.parseExpr(ser, vm, depth+1)
		if !childOK {
			d.warnf("failed to parse EX_JumpIfNot condition at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = nextSer, nextVM

	case 0x18: // EX_Skip
		skip, ok := d.readUint32(&ser, &vm, 4)
		if !ok {
			d.warnf("truncated EX_Skip at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("skipVmOffset", int(skip))
		nextSer, nextVM, _, childOK := d.parseExpr(ser, vm, depth+1)
		if !childOK {
			d.warnf("failed to parse EX_Skip payload at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = nextSer, nextVM

	case 0x5B: // EX_SkipOffsetConst
		skip, ok := d.readUint32(&ser, &vm, 4)
		if !ok {
			d.warnf("truncated EX_SkipOffsetConst at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("skipVmOffset", int(skip))

	case 0x4C: // EX_PushExecutionFlow
		target, ok := d.readUint32(&ser, &vm, 4)
		if !ok {
			d.warnf("truncated EX_PushExecutionFlow at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("targetVmOffset", int(target))

	case 0x4F: // EX_PopExecutionFlowIfNot
		setParam("conditionVmOffset", vm)
		nextSer, nextVM, _, childOK := d.parseExpr(ser, vm, depth+1)
		if !childOK {
			d.warnf("failed to parse EX_PopExecutionFlowIfNot condition at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = nextSer, nextVM

	case 0x04: // EX_Return
		nextSer, nextVM, _, childOK := d.parseExpr(ser, vm, depth+1)
		if !childOK {
			d.warnf("failed to parse EX_Return payload at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = nextSer, nextVM

	case 0x1B, 0x45: // EX_VirtualFunction / EX_LocalVirtualFunction
		ref, ok := d.readScriptName(&ser, &vm)
		if !ok {
			d.warnf("truncated %s function name at 0x%04X", inst.Opcode, inst.Offset)
			return ser, vm, token, false
		}
		setParam("functionName", ref.Display(d.names))
		paramCount, roots, listOK := d.parseExprListUntilWithRoots(&ser, &vm, depth+1, 0x16)
		setParam("paramCount", paramCount)
		setParam("paramExprVmOffsets", roots)
		if !listOK {
			d.warnf("%s parameter list does not terminate with EX_EndFunctionParms", inst.Opcode)
			return ser, vm, token, false
		}

	case 0x1C, 0x46, 0x63, 0x68: // final calls
		idx, ok := d.readObjectPointer(&ser, &vm)
		if !ok {
			d.warnf("truncated %s function pointer at 0x%04X", inst.Opcode, inst.Offset)
			return ser, vm, token, false
		}
		setParam("functionIndex", idx)
		if resolved := d.resolvePackageIndex(idx); resolved != "" {
			setParam("functionResolved", resolved)
		}
		paramCount, roots, listOK := d.parseExprListUntilWithRoots(&ser, &vm, depth+1, 0x16)
		setParam("paramCount", paramCount)
		setParam("paramExprVmOffsets", roots)
		if !listOK {
			d.warnf("%s parameter list does not terminate with EX_EndFunctionParms", inst.Opcode)
			return ser, vm, token, false
		}

	case 0x12, 0x19, 0x1A: // EX_ClassContext / EX_Context / EX_Context_FailSilent
		nextSer, nextVM, _, objectOK := d.parseExpr(ser, vm, depth+1)
		if !objectOK {
			d.warnf("failed to parse %s object expression at 0x%04X", inst.Opcode, inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = nextSer, nextVM
		target, ok := d.readUint32(&ser, &vm, 4)
		if !ok {
			d.warnf("truncated %s null-skip offset at 0x%04X", inst.Opcode, inst.Offset)
			return ser, vm, token, false
		}
		setParam("nullSkipVmOffset", int(target))
		rvalue, rvOK := d.readFieldPath(&ser, &vm)
		if !rvOK {
			d.warnf("truncated %s rvalue field path at 0x%04X", inst.Opcode, inst.Offset)
			return ser, vm, token, false
		}
		setParam("rValueFieldPath", rvalue)
		nextSer, nextVM, _, contextOK := d.parseExpr(ser, vm, depth+1)
		if !contextOK {
			d.warnf("failed to parse %s context expression at 0x%04X", inst.Opcode, inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = nextSer, nextVM

	case 0x0F: // EX_Let
		fieldPath, ok := d.readFieldPath(&ser, &vm)
		if !ok {
			d.warnf("truncated EX_Let destination field path at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("destinationFieldPath", fieldPath)
		fallthrough
	case 0x14, 0x5F, 0x60, 0x44, 0x43: // let variants
		leftSer, leftVM, _, leftOK := d.parseExpr(ser, vm, depth+1)
		if !leftOK {
			d.warnf("failed to parse %s lhs at 0x%04X", inst.Opcode, inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = leftSer, leftVM
		rightSer, rightVM, _, rightOK := d.parseExpr(ser, vm, depth+1)
		if !rightOK {
			d.warnf("failed to parse %s rhs at 0x%04X", inst.Opcode, inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = rightSer, rightVM

	case 0x64: // EX_LetValueOnPersistentFrame
		fieldPath, ok := d.readFieldPath(&ser, &vm)
		if !ok {
			d.warnf("truncated EX_LetValueOnPersistentFrame destination at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("destinationFieldPath", fieldPath)
		setParam("rhsVmOffset", vm)
		nextSer, nextVM, _, rhsOK := d.parseExpr(ser, vm, depth+1)
		if !rhsOK {
			d.warnf("failed to parse EX_LetValueOnPersistentFrame rhs at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = nextSer, nextVM

	case 0x42: // EX_StructMemberContext
		fieldPath, ok := d.readFieldPath(&ser, &vm)
		if !ok {
			d.warnf("truncated EX_StructMemberContext member field path at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("memberFieldPath", fieldPath)
		nextSer, nextVM, _, exprOK := d.parseExpr(ser, vm, depth+1)
		if !exprOK {
			d.warnf("failed to parse EX_StructMemberContext object expression at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = nextSer, nextVM

	case 0x38: // EX_Cast
		castType, ok := d.readUint8(&ser, &vm, 1)
		if !ok {
			d.warnf("truncated EX_Cast cast type at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("castType", castType)
		nextSer, nextVM, _, exprOK := d.parseExpr(ser, vm, depth+1)
		if !exprOK {
			d.warnf("failed to parse EX_Cast operand at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = nextSer, nextVM

	case 0x13, 0x2E, 0x52, 0x54, 0x55: // class cast variants
		idx, ok := d.readObjectPointer(&ser, &vm)
		if !ok {
			d.warnf("truncated %s class pointer at 0x%04X", inst.Opcode, inst.Offset)
			return ser, vm, token, false
		}
		setParam("classIndex", idx)
		if resolved := d.resolvePackageIndex(idx); resolved != "" {
			setParam("classResolved", resolved)
		}
		nextSer, nextVM, _, exprOK := d.parseExpr(ser, vm, depth+1)
		if !exprOK {
			d.warnf("failed to parse %s operand at 0x%04X", inst.Opcode, inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = nextSer, nextVM

	case 0x4B: // EX_InstanceDelegate
		ref, ok := d.readScriptName(&ser, &vm)
		if !ok {
			d.warnf("truncated EX_InstanceDelegate function name at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("functionName", ref.Display(d.names))

	case 0x61: // EX_BindDelegate
		ref, ok := d.readScriptName(&ser, &vm)
		if !ok {
			d.warnf("truncated EX_BindDelegate function name at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("functionName", ref.Display(d.names))
		firstSer, firstVM, _, firstOK := d.parseExpr(ser, vm, depth+1)
		if !firstOK {
			d.warnf("failed to parse EX_BindDelegate lhs at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = firstSer, firstVM
		secondSer, secondVM, _, secondOK := d.parseExpr(ser, vm, depth+1)
		if !secondOK {
			d.warnf("failed to parse EX_BindDelegate rhs at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = secondSer, secondVM

	case 0x5C, 0x62: // delegate add/remove
		firstSer, firstVM, _, firstOK := d.parseExpr(ser, vm, depth+1)
		if !firstOK {
			d.warnf("failed to parse %s lhs at 0x%04X", inst.Opcode, inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = firstSer, firstVM
		secondSer, secondVM, _, secondOK := d.parseExpr(ser, vm, depth+1)
		if !secondOK {
			d.warnf("failed to parse %s rhs at 0x%04X", inst.Opcode, inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = secondSer, secondVM

	case 0x5D: // EX_ClearMulticastDelegate
		nextSer, nextVM, _, childOK := d.parseExpr(ser, vm, depth+1)
		if !childOK {
			d.warnf("failed to parse EX_ClearMulticastDelegate target at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = nextSer, nextVM

	case 0x69: // EX_SwitchValue
		numCases, ok := d.readUint16(&ser, &vm, 2)
		if !ok {
			d.warnf("truncated EX_SwitchValue case count at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("numCases", int(numCases))
		doneTarget, doneOK := d.readUint32(&ser, &vm, 4)
		if !doneOK {
			d.warnf("truncated EX_SwitchValue done offset at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("switchDoneVmOffset", int(doneTarget))

		setParam("indexTermVmOffset", vm)
		indexSer, indexVM, _, indexOK := d.parseExpr(ser, vm, depth+1)
		if !indexOK {
			d.warnf("failed to parse EX_SwitchValue index term at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = indexSer, indexVM

		for i := 0; i < int(numCases); i++ {
			setParam(fmt.Sprintf("case_%d_keyVmOffset", i), vm)
			keySer, keyVM, _, keyOK := d.parseExpr(ser, vm, depth+1)
			if !keyOK {
				d.warnf("failed to parse EX_SwitchValue case[%d] key at 0x%04X", i, inst.Offset)
				return ser, vm, token, false
			}
			ser, vm = keySer, keyVM

			nextCase, nextCaseOK := d.readUint32(&ser, &vm, 4)
			if !nextCaseOK {
				d.warnf("truncated EX_SwitchValue case[%d] next offset at 0x%04X", i, inst.Offset)
				return ser, vm, token, false
			}
			setParam(fmt.Sprintf("case_%d_nextVmOffset", i), int(nextCase))

			setParam(fmt.Sprintf("case_%d_bodyVmOffset", i), vm)
			bodySer, bodyVM, _, bodyOK := d.parseExpr(ser, vm, depth+1)
			if !bodyOK {
				d.warnf("failed to parse EX_SwitchValue case[%d] body at 0x%04X", i, inst.Offset)
				return ser, vm, token, false
			}
			ser, vm = bodySer, bodyVM
		}

		setParam("defaultVmOffset", vm)
		defSer, defVM, _, defOK := d.parseExpr(ser, vm, depth+1)
		if !defOK {
			d.warnf("failed to parse EX_SwitchValue default at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = defSer, defVM

	case 0x31: // EX_SetArray
		headSer, headVM, _, headOK := d.parseExpr(ser, vm, depth+1)
		if !headOK {
			d.warnf("failed to parse EX_SetArray target at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = headSer, headVM
		elementCount, listOK := d.parseExprListUntil(&ser, &vm, depth+1, 0x32)
		setParam("elementCount", elementCount)
		if !listOK {
			d.warnf("EX_SetArray list does not terminate with EX_EndArray")
			return ser, vm, token, false
		}

	case 0x39: // EX_SetSet
		headSer, headVM, _, headOK := d.parseExpr(ser, vm, depth+1)
		if !headOK {
			d.warnf("failed to parse EX_SetSet target at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = headSer, headVM
		declaredCount, countOK := d.readInt32(&ser, &vm, 4)
		if !countOK {
			d.warnf("truncated EX_SetSet declared count at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("declaredElementCount", declaredCount)
		elementCount, listOK := d.parseExprListUntil(&ser, &vm, depth+1, 0x3A)
		setParam("elementCount", elementCount)
		if !listOK {
			d.warnf("EX_SetSet list does not terminate with EX_EndSet")
			return ser, vm, token, false
		}

	case 0x3B: // EX_SetMap
		headSer, headVM, _, headOK := d.parseExpr(ser, vm, depth+1)
		if !headOK {
			d.warnf("failed to parse EX_SetMap target at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = headSer, headVM
		declaredCount, countOK := d.readInt32(&ser, &vm, 4)
		if !countOK {
			d.warnf("truncated EX_SetMap declared count at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("declaredElementCount", declaredCount)
		elementCount, listOK := d.parseExprListUntil(&ser, &vm, depth+1, 0x3C)
		setParam("elementCount", elementCount)
		if !listOK {
			d.warnf("EX_SetMap list does not terminate with EX_EndMap")
			return ser, vm, token, false
		}

	case 0x65: // EX_ArrayConst
		fieldPath, ok := d.readFieldPath(&ser, &vm)
		if !ok {
			d.warnf("truncated EX_ArrayConst inner property at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("innerProperty", fieldPath)
		declaredCount, countOK := d.readInt32(&ser, &vm, 4)
		if !countOK {
			d.warnf("truncated EX_ArrayConst declared count at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("declaredElementCount", declaredCount)
		elementCount, listOK := d.parseExprListUntil(&ser, &vm, depth+1, 0x66)
		setParam("elementCount", elementCount)
		if !listOK {
			d.warnf("EX_ArrayConst list does not terminate with EX_EndArrayConst")
			return ser, vm, token, false
		}

	case 0x3D: // EX_SetConst
		fieldPath, ok := d.readFieldPath(&ser, &vm)
		if !ok {
			d.warnf("truncated EX_SetConst inner property at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("innerProperty", fieldPath)
		declaredCount, countOK := d.readInt32(&ser, &vm, 4)
		if !countOK {
			d.warnf("truncated EX_SetConst declared count at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("declaredElementCount", declaredCount)
		elementCount, listOK := d.parseExprListUntil(&ser, &vm, depth+1, 0x3E)
		setParam("elementCount", elementCount)
		if !listOK {
			d.warnf("EX_SetConst list does not terminate with EX_EndSetConst")
			return ser, vm, token, false
		}

	case 0x3F: // EX_MapConst
		keyPath, keyOK := d.readFieldPath(&ser, &vm)
		if !keyOK {
			d.warnf("truncated EX_MapConst key property at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		valuePath, valueOK := d.readFieldPath(&ser, &vm)
		if !valueOK {
			d.warnf("truncated EX_MapConst value property at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("keyProperty", keyPath)
		setParam("valueProperty", valuePath)
		declaredCount, countOK := d.readInt32(&ser, &vm, 4)
		if !countOK {
			d.warnf("truncated EX_MapConst declared count at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("declaredElementCount", declaredCount)
		elementCount, listOK := d.parseExprListUntil(&ser, &vm, depth+1, 0x40)
		setParam("elementCount", elementCount)
		if !listOK {
			d.warnf("EX_MapConst list does not terminate with EX_EndMapConst")
			return ser, vm, token, false
		}

	case 0x11: // EX_BitFieldConst
		fieldPath, ok := d.readFieldPath(&ser, &vm)
		if !ok {
			d.warnf("truncated EX_BitFieldConst property at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("bitProperty", fieldPath)
		bitValue, bitOK := d.readUint8(&ser, &vm, 1)
		if !bitOK {
			d.warnf("truncated EX_BitFieldConst value at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("value", bitValue)

	case 0x2F: // EX_StructConst
		structIdx, idxOK := d.readObjectPointer(&ser, &vm)
		if !idxOK {
			d.warnf("truncated EX_StructConst struct pointer at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("structIndex", structIdx)
		if resolved := d.resolvePackageIndex(structIdx); resolved != "" {
			setParam("structResolved", resolved)
		}
		serializedSize, sizeOK := d.readInt32(&ser, &vm, 4)
		if !sizeOK {
			d.warnf("truncated EX_StructConst serialized size at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("serializedSize", serializedSize)
		elementCount, listOK := d.parseExprListUntil(&ser, &vm, depth+1, 0x30)
		setParam("elementCount", elementCount)
		if !listOK {
			d.warnf("EX_StructConst body does not terminate with EX_EndStructConst")
			return ser, vm, token, false
		}

	case 0x09: // EX_Assert
		line, lineOK := d.readUint16(&ser, &vm, 2)
		if !lineOK {
			d.warnf("truncated EX_Assert line number at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("line", line)
		mode, modeOK := d.readUint8(&ser, &vm, 1)
		if !modeOK {
			d.warnf("truncated EX_Assert mode at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("mode", mode)
		nextSer, nextVM, _, exprOK := d.parseExpr(ser, vm, depth+1)
		if !exprOK {
			d.warnf("failed to parse EX_Assert expression at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = nextSer, nextVM

	case 0x29: // EX_TextConst
		literalType, ok := d.readUint8(&ser, &vm, 1)
		if !ok {
			d.warnf("truncated EX_TextConst literal type at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("literalType", literalType)
		switch literalType {
		case 0: // Empty
		case 1: // LocalizedText
			for i := 0; i < 3; i++ {
				nextSer, nextVM, _, exprOK := d.parseExpr(ser, vm, depth+1)
				if !exprOK {
					d.warnf("failed to parse EX_TextConst LocalizedText operand[%d] at 0x%04X", i, inst.Offset)
					return ser, vm, token, false
				}
				ser, vm = nextSer, nextVM
			}
		case 2, 3: // InvariantText / LiteralString
			nextSer, nextVM, _, exprOK := d.parseExpr(ser, vm, depth+1)
			if !exprOK {
				d.warnf("failed to parse EX_TextConst operand at 0x%04X", inst.Offset)
				return ser, vm, token, false
			}
			ser, vm = nextSer, nextVM
		case 4: // StringTableEntry
			idx, idxOK := d.readObjectPointer(&ser, &vm)
			if !idxOK {
				d.warnf("truncated EX_TextConst string table object pointer at 0x%04X", inst.Offset)
				return ser, vm, token, false
			}
			setParam("stringTableObjectIndex", idx)
			if resolved := d.resolvePackageIndex(idx); resolved != "" {
				setParam("stringTableObjectResolved", resolved)
			}
			for i := 0; i < 2; i++ {
				nextSer, nextVM, _, exprOK := d.parseExpr(ser, vm, depth+1)
				if !exprOK {
					d.warnf("failed to parse EX_TextConst StringTableEntry operand[%d] at 0x%04X", i, inst.Offset)
					return ser, vm, token, false
				}
				ser, vm = nextSer, nextVM
			}
		default:
			d.warnf("unknown EX_TextConst literal type=%d", literalType)
		}

	case 0x22: // EX_RotationConst (UE5.6 = double x3)
		if !d.skipSerializedBytes(&ser, &vm, 24, 24) {
			d.warnf("truncated EX_RotationConst at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}

	case 0x23: // EX_VectorConst (UE5.6 = double x3)
		if !d.skipSerializedBytes(&ser, &vm, 24, 24) {
			d.warnf("truncated EX_VectorConst at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}

	case 0x41: // EX_Vector3fConst (float x3)
		if !d.skipSerializedBytes(&ser, &vm, 12, 12) {
			d.warnf("truncated EX_Vector3fConst at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}

	case 0x2B: // EX_TransformConst (UE5.6 = double x10)
		if !d.skipSerializedBytes(&ser, &vm, 80, 80) {
			d.warnf("truncated EX_TransformConst at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}

	case 0x6A: // EX_InstrumentationEvent
		eventType, ok := d.readUint8(&ser, &vm, 1)
		if !ok {
			d.warnf("truncated EX_InstrumentationEvent at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("eventType", eventType)
		// InlineEvent carries one FScriptName payload.
		if eventType == inlineInstrumentationEventType {
			ref, nameOK := d.readScriptName(&ser, &vm)
			if !nameOK {
				d.warnf("truncated EX_InstrumentationEvent inline name at 0x%04X", inst.Offset)
				return ser, vm, token, false
			}
			setParam("eventName", ref.Display(d.names))
		}

	case 0x0C: // EX_NothingInt32
		if _, ok := d.readInt32(&ser, &vm, 4); !ok {
			d.warnf("truncated EX_NothingInt32 at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}

	case 0x70: // EX_AutoRtfmTransact
		txID, ok := d.readInt32(&ser, &vm, 4)
		if !ok {
			d.warnf("truncated EX_AutoRtfmTransact transaction id at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("transactionId", txID)
		target, targetOK := d.readUint32(&ser, &vm, 4)
		if !targetOK {
			d.warnf("truncated EX_AutoRtfmTransact target offset at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("targetVmOffset", int(target))
		paramCount, listOK := d.parseExprListUntil(&ser, &vm, depth+1, 0x71)
		setParam("paramCount", paramCount)
		if !listOK {
			d.warnf("EX_AutoRtfmTransact list does not terminate with EX_AutoRtfmStopTransact")
			return ser, vm, token, false
		}

	case 0x71: // EX_AutoRtfmStopTransact
		txID, ok := d.readInt32(&ser, &vm, 4)
		if !ok {
			d.warnf("truncated EX_AutoRtfmStopTransact transaction id at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("transactionId", txID)
		mode, modeOK := d.readUint8(&ser, &vm, 1)
		if !modeOK {
			d.warnf("truncated EX_AutoRtfmStopTransact mode at 0x%04X", inst.Offset)
			return ser, vm, token, false
		}
		setParam("stopMode", mode)

	case 0x4E: // EX_ComputedJump
		setParam("operandVmOffset", vm)
		firstSer, firstVM, _, firstOK := d.parseExpr(ser, vm, depth+1)
		if !firstOK {
			d.warnf("failed to parse %s operand at 0x%04X", inst.Opcode, inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = firstSer, firstVM

	case 0x51, 0x67, 0x6B, 0x6D, 0x72: // single-expression operands
		firstSer, firstVM, _, firstOK := d.parseExpr(ser, vm, depth+1)
		if !firstOK {
			d.warnf("failed to parse %s operand at 0x%04X", inst.Opcode, inst.Offset)
			return ser, vm, token, false
		}
		ser, vm = firstSer, firstVM
		if token == 0x6B { // ArrayGetByRef has two expressions
			secondSer, secondVM, _, secondOK := d.parseExpr(ser, vm, depth+1)
			if !secondOK {
				d.warnf("failed to parse EX_ArrayGetByRef second operand at 0x%04X", inst.Offset)
				return ser, vm, token, false
			}
			ser, vm = secondSer, secondVM
		}

	case 0x16, 0x53, 0x30, 0x32, 0x3A, 0x3C, 0x66, 0x3E, 0x40,
		0x0B, 0x25, 0x26, 0x27, 0x28, 0x2A, 0x2D, 0x17, 0x15, 0x4D, 0x4A,
		0x5A, 0x5E, 0x50:
		// no payload
	}

	return ser, vm, token, true
}

func (d *serializedDisasmDecoder) parseExprListUntil(ser, vm *int, depth int, terminator uint8) (int, bool) {
	count, _, ok := d.parseExprListUntilWithRoots(ser, vm, depth, terminator)
	return count, ok
}

func (d *serializedDisasmDecoder) parseExprListUntilWithRoots(ser, vm *int, depth int, terminator uint8) (int, []int, bool) {
	count := 0
	roots := make([]int, 0, 4)
	guard := len(d.data) * 4
	if guard < 64 {
		guard = 64
	}
	for *ser < len(d.data) && guard > 0 {
		currentVM := *vm
		nextSer, nextVM, token, ok := d.parseExpr(*ser, *vm, depth+1)
		if !ok {
			return count, roots, false
		}
		*ser, *vm = nextSer, nextVM
		guard--
		if token == terminator {
			return count, roots, true
		}
		roots = append(roots, currentVM)
		count++
	}
	return count, roots, false
}

func (d *serializedDisasmDecoder) readFieldPath(ser, vm *int) (map[string]any, bool) {
	vmBase := *vm
	countRaw, ok := d.readInt32(ser, vm, 0)
	if !ok {
		return nil, false
	}
	if countRaw < 0 || countRaw > maxFieldPathSegments {
		return nil, false
	}
	count := int(countRaw)
	segments := make([]string, 0, count)
	for i := 0; i < count; i++ {
		ref, nameOK := d.readScriptName(ser, vm)
		if !nameOK {
			return nil, false
		}
		segments = append(segments, ref.Display(d.names))
	}
	ownerIndex, ownerOK := d.readInt32(ser, vm, 0)
	if !ownerOK {
		return nil, false
	}
	// UE VM treats property/field references as fixed ScriptPointerType-sized payloads
	// in iCode progression, even though serialized asset representation is a variable-length field path.
	*vm = vmBase + virtualScriptPointerSize
	out := map[string]any{
		"segments":   segments,
		"ownerIndex": ownerIndex,
	}
	if resolved := d.resolvePackageIndex(ownerIndex); resolved != "" {
		out["ownerResolved"] = resolved
	}
	return out, true
}

func (d *serializedDisasmDecoder) readScriptName(ser, vm *int) (uasset.NameRef, bool) {
	if *ser+8 > len(d.data) {
		return uasset.NameRef{}, false
	}
	idx := int32(binary.LittleEndian.Uint32(d.data[*ser : *ser+4]))
	num := int32(binary.LittleEndian.Uint32(d.data[*ser+4 : *ser+8]))
	*ser += 8
	*vm += 8
	return uasset.NameRef{Index: idx, Number: num}, true
}

func (d *serializedDisasmDecoder) readObjectPointer(ser, vm *int) (int32, bool) {
	return d.readInt32(ser, vm, virtualScriptPointerSize)
}

func (d *serializedDisasmDecoder) readUint8(ser, vm *int, vmAdvance int) (uint8, bool) {
	if *ser+1 > len(d.data) {
		return 0, false
	}
	v := d.data[*ser]
	*ser += 1
	*vm += vmAdvance
	return v, true
}

func (d *serializedDisasmDecoder) readUint16(ser, vm *int, vmAdvance int) (uint16, bool) {
	if *ser+2 > len(d.data) {
		return 0, false
	}
	v := binary.LittleEndian.Uint16(d.data[*ser : *ser+2])
	*ser += 2
	*vm += vmAdvance
	return v, true
}

func (d *serializedDisasmDecoder) readUint32(ser, vm *int, vmAdvance int) (uint32, bool) {
	if *ser+4 > len(d.data) {
		return 0, false
	}
	v := binary.LittleEndian.Uint32(d.data[*ser : *ser+4])
	*ser += 4
	*vm += vmAdvance
	return v, true
}

func (d *serializedDisasmDecoder) readInt32(ser, vm *int, vmAdvance int) (int32, bool) {
	v, ok := d.readUint32(ser, vm, vmAdvance)
	return int32(v), ok
}

func (d *serializedDisasmDecoder) readUint64(ser, vm *int, vmAdvance int) (uint64, bool) {
	if *ser+8 > len(d.data) {
		return 0, false
	}
	v := binary.LittleEndian.Uint64(d.data[*ser : *ser+8])
	*ser += 8
	*vm += vmAdvance
	return v, true
}

func (d *serializedDisasmDecoder) readInt64(ser, vm *int, vmAdvance int) (int64, bool) {
	v, ok := d.readUint64(ser, vm, vmAdvance)
	return int64(v), ok
}

func (d *serializedDisasmDecoder) skipSerializedBytes(ser, vm *int, byteSize, vmAdvance int) bool {
	if *ser+byteSize > len(d.data) {
		return false
	}
	*ser += byteSize
	*vm += vmAdvance
	return true
}

func (d *serializedDisasmDecoder) resolvePackageIndex(raw int32) string {
	if d.asset == nil {
		return ""
	}
	return d.asset.ParseIndex(uasset.PackageIndex(raw))
}

func (d *serializedDisasmDecoder) warnf(format string, args ...any) {
	if len(d.warnings) >= maxDisasmWarnings {
		return
	}
	d.warnings = append(d.warnings, fmt.Sprintf(format, args...))
}
