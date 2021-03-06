// Copyright (C) 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gles

import (
	"github.com/google/gapid/core/log"
	"github.com/google/gapid/gapis/atom"
	"github.com/google/gapid/gapis/atom/transform"
	"github.com/google/gapid/gapis/gfxapi"
	"github.com/google/gapid/gapis/memory"
)

// tweaker provides a set of methods for temporarily changing the GLES state.
type tweaker struct {
	ctx  log.Context // Needed so functions match gl signature
	out  transform.Writer
	s    *gfxapi.State
	c    *Context
	undo []func()
}

func newTweaker(ctx log.Context, out transform.Writer) *tweaker {
	s := out.State()
	c := GetContext(s)
	return &tweaker{ctx: ctx, out: out, s: s, c: c}
}

// revert undoes all the changes made by the tweaker.
func (t *tweaker) revert() {
	for i := len(t.undo) - 1; i >= 0; i-- {
		t.undo[i]()
	}
	t.undo = nil
}

func (t *tweaker) doAndUndo(do atom.Atom, undo atom.Atom) {
	t.out.MutateAndWrite(t.ctx, atom.NoID, do)
	t.undo = append(t.undo, func() {
		t.out.MutateAndWrite(t.ctx, atom.NoID, undo)
	})
}

func (t *tweaker) AllocData(v ...interface{}) atom.AllocResult {
	tmp := atom.Must(atom.AllocData(t.ctx, t.s, v...))
	t.undo = append(t.undo, func() {
		tmp.Free()
	})
	return tmp
}

func (t *tweaker) getCapability(name GLenum) bool {
	a := NewGlIsEnabled(name, 0)
	o := a.Extras().Observations()
	s := t.out.State()
	i := GLuint(0) // capability index.
	res, err := subGetCapability(t.ctx, a, o, s, GetState(s), nil, name, i)
	if err != nil {
		panic(err)
	}
	return res != 0
}

func (t *tweaker) glEnable(name GLenum) {
	// TODO: This does not correctly handle indexed state.
	if o := t.getCapability(name); o != true {
		t.doAndUndo(
			NewGlEnable(name),
			NewGlDisable(name))
	}
}

func (t *tweaker) glDisable(name GLenum) {
	// TODO: This does not correctly handle indexed state.
	if o := t.getCapability(name); o != false {
		t.doAndUndo(
			NewGlDisable(name),
			NewGlEnable(name))
	}
}

func (t *tweaker) glDepthMask(v GLboolean) {
	if o := t.c.Framebuffer.DepthWritemask; o != v {
		t.doAndUndo(
			NewGlDepthMask(v),
			NewGlDepthMask(o))
	}
}

func (t *tweaker) glDepthFunc(v GLenum) {
	if o := t.c.FragmentOperations.Depth.Func; o != v {
		t.doAndUndo(
			NewGlDepthFunc(v),
			NewGlDepthFunc(o))
	}
}

func (t *tweaker) glBlendColor(r, g, b, a GLfloat) {
	n := Color{Red: r, Green: g, Blue: b, Alpha: a}
	if o := t.c.FragmentOperations.BlendColor; o != n {
		t.doAndUndo(
			NewGlBlendColor(r, g, b, a),
			NewGlBlendColor(o.Red, o.Green, o.Blue, o.Alpha))
	}
}

func (t *tweaker) glBlendFunc(src, dst GLenum) {
	t.glBlendFuncSeparate(src, dst, src, dst)
}

func (t *tweaker) glBlendFuncSeparate(srcRGB, dstRGB, srcA, dstA GLenum) {
	// TODO: This does not correctly handle indexed state.
	o := t.c.FragmentOperations.Blend[0]
	n := o
	n.SrcRgb, n.DstRgb, n.SrcAlpha, n.DstAlpha = srcRGB, dstRGB, srcA, dstA
	if o != n {
		t.doAndUndo(
			NewGlBlendFuncSeparate(srcRGB, dstRGB, srcA, dstA),
			NewGlBlendFuncSeparate(o.SrcRgb, o.DstRgb, o.SrcAlpha, o.DstAlpha))
	}
}

// glPolygonOffset adjusts the offset depth factor and units. Unlike the original glPolygonOffset,
// this function adds the given values to the current values rather than setting them.
func (t *tweaker) glPolygonOffset(factor, units GLfloat) {
	origFactor, origUnits := t.c.Rasterization.PolygonOffsetFactor, t.c.Rasterization.PolygonOffsetUnits
	t.doAndUndo(
		NewGlPolygonOffset(origFactor+factor, origUnits+units),
		NewGlPolygonOffset(origFactor, origUnits))
}

func (t *tweaker) glLineWidth(width GLfloat) {
	if o := t.c.Rasterization.LineWidth; o != width {
		t.doAndUndo(
			NewGlLineWidth(width),
			NewGlLineWidth(o))
	}
}

// This will either bind new VAO (GLES 3.x) or save state of the default one (GLES 2.0).
func (t *tweaker) makeVertexArray(enabledLocations ...AttributeLocation) {
	ctx := t.ctx
	if t.c.Constants.MajorVersion >= 3 {
		// GLES 3.0 and 3.1 introduce a lot of new state which would be hard to restore.
		// It is much easier to just create a fresh Vertex Array Object to work with.
		vertexArrayID := t.glGenVertexArray()
		t.glBindVertexArray(vertexArrayID)
		for _, location := range enabledLocations {
			t.out.MutateAndWrite(ctx, atom.NoID, NewGlEnableVertexAttribArray(location))
		}
	} else {
		// GLES 2.0 does not have Vertex Array Objects, but the state is fairly simple.
		vao := t.c.Instances.VertexArrays[t.c.BoundVertexArray]
		// Disable all vertex attribute arrays
		for location, origVertexAttrib := range vao.VertexAttributeArrays {
			if origVertexAttrib.Enabled == GLboolean_GL_TRUE {
				t.doAndUndo(
					NewGlDisableVertexAttribArray(location),
					NewGlEnableVertexAttribArray(location))
			}
		}
		// Enable and save state for the attribute arrays that we will use
		origArrayBufferID := t.c.BoundBuffers.ArrayBuffer
		for _, location := range enabledLocations {
			location := location
			t.doAndUndo(
				NewGlEnableVertexAttribArray(location),
				NewGlDisableVertexAttribArray(location))
			origVertexAttrib := *(vao.VertexAttributeArrays[location])
			origVertexBinding := *(vao.VertexBufferBindings[VertexBufferBindingIndex(location)])
			t.undo = append(t.undo, func() {
				t.out.MutateAndWrite(ctx, atom.NoID, NewGlBindBuffer(GLenum_GL_ARRAY_BUFFER, origVertexBinding.Buffer))
				t.out.MutateAndWrite(ctx, atom.NoID, NewGlVertexAttribPointer(location, origVertexAttrib.Size, origVertexAttrib.Type, origVertexAttrib.Normalized, origVertexAttrib.Stride, memory.Pointer(origVertexAttrib.Pointer)))
				t.out.MutateAndWrite(ctx, atom.NoID, NewGlBindBuffer(GLenum_GL_ARRAY_BUFFER, origArrayBufferID))
			})
		}
	}
}

func (t *tweaker) glGenBuffer() BufferId {
	id := BufferId(newUnusedID('B', func(x uint32) bool { return t.c.Instances.Buffers[BufferId(x)] != nil }))
	tmp := t.AllocData(id)
	t.doAndUndo(
		NewGlGenBuffers(1, tmp.Ptr()).AddWrite(tmp.Data()),
		NewGlDeleteBuffers(1, tmp.Ptr()).AddRead(tmp.Data()))
	return id
}

func (t *tweaker) glGenRenderbuffer() RenderbufferId {
	id := RenderbufferId(newUnusedID('R', func(x uint32) bool { return t.c.Instances.Renderbuffers[RenderbufferId(x)] != nil }))
	tmp := t.AllocData(id)
	t.doAndUndo(
		NewGlGenRenderbuffers(1, tmp.Ptr()).AddWrite(tmp.Data()),
		NewGlDeleteRenderbuffers(1, tmp.Ptr()).AddRead(tmp.Data()))
	return id
}

func (t *tweaker) glGenFramebuffer() FramebufferId {
	id := FramebufferId(newUnusedID('F', func(x uint32) bool { return t.c.Instances.Framebuffers[FramebufferId(x)] != nil }))
	tmp := t.AllocData(id)
	t.doAndUndo(
		NewGlGenFramebuffers(1, tmp.Ptr()).AddWrite(tmp.Data()),
		NewGlDeleteFramebuffers(1, tmp.Ptr()).AddRead(tmp.Data()))
	return id
}

func (t *tweaker) glGenTexture() TextureId {
	id := TextureId(newUnusedID('T', func(x uint32) bool { return t.c.Instances.Textures[TextureId(x)] != nil }))
	tmp := t.AllocData(id)
	t.doAndUndo(
		NewGlGenTextures(1, tmp.Ptr()).AddWrite(tmp.Data()),
		NewGlDeleteTextures(1, tmp.Ptr()).AddRead(tmp.Data()))
	return id
}

func (t *tweaker) glGenVertexArray() VertexArrayId {
	id := VertexArrayId(newUnusedID('V', func(x uint32) bool { return t.c.Instances.VertexArrays[VertexArrayId(x)] != nil }))
	tmp := t.AllocData(id)
	t.doAndUndo(
		NewGlGenVertexArrays(1, tmp.Ptr()).AddWrite(tmp.Data()),
		NewGlDeleteVertexArrays(1, tmp.Ptr()).AddRead(tmp.Data()))
	return id
}

func (t *tweaker) glCreateProgram() ProgramId {
	id := ProgramId(newUnusedID('P', func(x uint32) bool {
		return t.c.Instances.Programs[ProgramId(x)] != nil || t.c.Instances.Shaders[ShaderId(x)] != nil
	}))
	t.doAndUndo(
		NewGlCreateProgram(id),
		NewGlDeleteProgram(id))
	return id
}

func (t *tweaker) makeProgram(vertexShaderSource, fragmentShaderSource string) ProgramId {
	programID := t.glCreateProgram()
	vertexShaderID := t.glCreateShader(GLenum_GL_VERTEX_SHADER)
	t.glShaderSource(vertexShaderID, vertexShaderSource)
	t.out.MutateAndWrite(t.ctx, atom.NoID, NewGlCompileShader(vertexShaderID))
	fragmentShaderID := t.glCreateShader(GLenum_GL_FRAGMENT_SHADER)
	t.glShaderSource(fragmentShaderID, fragmentShaderSource)
	t.out.MutateAndWrite(t.ctx, atom.NoID, NewGlCompileShader(fragmentShaderID))
	t.out.MutateAndWrite(t.ctx, atom.NoID, NewGlAttachShader(programID, vertexShaderID))
	t.out.MutateAndWrite(t.ctx, atom.NoID, NewGlAttachShader(programID, fragmentShaderID))
	return programID
}

func (t *tweaker) glCreateShader(shaderType GLenum) ShaderId {
	id := ShaderId(newUnusedID('S', func(x uint32) bool {
		return t.c.Instances.Programs[ProgramId(x)] != nil || t.c.Instances.Shaders[ShaderId(x)] != nil
	}))
	// We need to mutate the state, as otherwise two consecutive calls can return the same ShaderId.
	t.doAndUndo(
		NewGlCreateShader(shaderType, id),
		NewGlDeleteShader(id))
	return id
}

func (t *tweaker) glShaderSource(shaderID ShaderId, shaderSource string) {
	tmpSrc := t.AllocData(shaderSource)
	tmpSrcLen := t.AllocData(GLint(len(shaderSource)))
	tmpPtrToSrc := t.AllocData(tmpSrc.Ptr())
	t.out.MutateAndWrite(t.ctx, atom.NoID, NewGlShaderSource(shaderID, 1, tmpPtrToSrc.Ptr(), tmpSrcLen.Ptr()).
		AddRead(tmpPtrToSrc.Data()).
		AddRead(tmpSrcLen.Data()).
		AddRead(tmpSrc.Data()))
	return
}

func (t *tweaker) glScissor(x, y GLint, w, h GLsizei) {
	v := Rect{X: x, Y: y, Width: w, Height: h}
	if o := t.c.FragmentOperations.Scissor.Box; o != v {
		t.doAndUndo(
			NewGlScissor(x, y, w, h),
			NewGlScissor(o.X, o.Y, o.Width, o.Height))
	}
}

func (t *tweaker) GlBindBuffer_ArrayBuffer(id BufferId) {
	if o := t.c.BoundBuffers.ArrayBuffer; o != id {
		t.doAndUndo(
			NewGlBindBuffer(GLenum_GL_ARRAY_BUFFER, id),
			NewGlBindBuffer(GLenum_GL_ARRAY_BUFFER, o))
	}
}

func (t *tweaker) GlBindBuffer_ElementArrayBuffer(id BufferId) {
	vao := t.c.Instances.VertexArrays[t.c.BoundVertexArray]
	if o := vao.ElementArrayBuffer; o != id {
		t.doAndUndo(
			NewGlBindBuffer(GLenum_GL_ELEMENT_ARRAY_BUFFER, id),
			NewGlBindBuffer(GLenum_GL_ELEMENT_ARRAY_BUFFER, o))
	}
}

func (t *tweaker) glBindFramebuffer_Draw(id FramebufferId) {
	if o := t.c.BoundDrawFramebuffer; o != id {
		t.doAndUndo(
			NewGlBindFramebuffer(GLenum_GL_DRAW_FRAMEBUFFER, id),
			NewGlBindFramebuffer(GLenum_GL_DRAW_FRAMEBUFFER, o))
	}
}

func (t *tweaker) glBindFramebuffer_Read(id FramebufferId) {
	if o := t.c.BoundReadFramebuffer; o != id {
		t.doAndUndo(
			NewGlBindFramebuffer(GLenum_GL_READ_FRAMEBUFFER, id),
			NewGlBindFramebuffer(GLenum_GL_READ_FRAMEBUFFER, o))
	}
}

func (t *tweaker) glReadBuffer(id GLenum) {
	fb := t.c.Instances.Framebuffers[t.c.BoundReadFramebuffer]
	if o := fb.ReadBuffer; o != id {
		t.doAndUndo(
			NewGlReadBuffer(id),
			NewGlReadBuffer(o))
	}
}

func (t *tweaker) glBindRenderbuffer(id RenderbufferId) {
	if o := t.c.BoundRenderbuffer; o != id {
		t.doAndUndo(
			NewGlBindRenderbuffer(GLenum_GL_RENDERBUFFER, id),
			NewGlBindRenderbuffer(GLenum_GL_RENDERBUFFER, o))
	}
}

func (t *tweaker) glBindTexture_2D(id TextureId) {
	if o := t.c.TextureUnits[t.c.ActiveTextureUnit].Binding2d; o != id {
		t.doAndUndo(
			NewGlBindTexture(GLenum_GL_TEXTURE_2D, id),
			NewGlBindTexture(GLenum_GL_TEXTURE_2D, o))
	}
}

func (t *tweaker) glBindVertexArray(id VertexArrayId) {
	if o := t.c.BoundVertexArray; o != id {
		t.doAndUndo(
			NewGlBindVertexArray(id),
			NewGlBindVertexArray(o))
	}
}

func (t *tweaker) glUseProgram(id ProgramId) {
	if o := t.c.BoundProgram; o != id {
		t.doAndUndo(
			NewGlUseProgram(id),
			NewGlUseProgram(o))
	}
}

func (t *tweaker) glActiveTexture(unit GLenum) {
	if o := t.c.ActiveTextureUnit; o != unit {
		t.doAndUndo(
			NewGlActiveTexture(unit),
			NewGlActiveTexture(o))
	}
}

func (t *tweaker) setPixelStorage(state PixelStorageState, packBufferId, unpackBufferId BufferId) {
	origState := map[GLenum]GLint{}
	forEachPixelStorageState(t.c.PixelStorage, func(n GLenum, v GLint) { origState[n] = v })
	forEachPixelStorageState(state, func(name GLenum, value GLint) {
		if o := origState[name]; o != value {
			t.doAndUndo(
				NewGlPixelStorei(name, value),
				NewGlPixelStorei(name, o))
		}
	})
	if o := t.c.BoundBuffers.PixelPackBuffer; o != packBufferId {
		t.doAndUndo(
			NewGlBindBuffer(GLenum_GL_PIXEL_PACK_BUFFER, packBufferId),
			NewGlBindBuffer(GLenum_GL_PIXEL_PACK_BUFFER, o))
	}
	if o := t.c.BoundBuffers.PixelUnpackBuffer; o != unpackBufferId {
		t.doAndUndo(
			NewGlBindBuffer(GLenum_GL_PIXEL_UNPACK_BUFFER, unpackBufferId),
			NewGlBindBuffer(GLenum_GL_PIXEL_UNPACK_BUFFER, o))
	}
}

func forEachPixelStorageState(state PixelStorageState, action func(n GLenum, v GLint)) {
	action(GLenum_GL_UNPACK_ALIGNMENT, state.UnpackAlignment)
	action(GLenum_GL_UNPACK_IMAGE_HEIGHT, state.UnpackImageHeight)
	action(GLenum_GL_UNPACK_ROW_LENGTH, state.UnpackRowLength)
	action(GLenum_GL_UNPACK_SKIP_IMAGES, state.UnpackSkipImages)
	action(GLenum_GL_UNPACK_SKIP_PIXELS, state.UnpackSkipPixels)
	action(GLenum_GL_UNPACK_SKIP_ROWS, state.UnpackSkipRows)
	action(GLenum_GL_PACK_ALIGNMENT, state.PackAlignment)
	action(GLenum_GL_PACK_IMAGE_HEIGHT, state.PackImageHeight)
	action(GLenum_GL_PACK_ROW_LENGTH, state.PackRowLength)
	action(GLenum_GL_PACK_SKIP_IMAGES, state.PackSkipImages)
	action(GLenum_GL_PACK_SKIP_PIXELS, state.PackSkipPixels)
	action(GLenum_GL_PACK_SKIP_ROWS, state.PackSkipRows)
}
