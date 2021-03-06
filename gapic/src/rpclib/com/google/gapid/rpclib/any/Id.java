/*
 * Copyright (C) 2017 Google Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package com.google.gapid.rpclib.any;

import com.google.gapid.rpclib.binary.BinaryClass;
import com.google.gapid.rpclib.binary.BinaryID;
import com.google.gapid.rpclib.binary.BinaryObject;
import com.google.gapid.rpclib.binary.Decoder;
import com.google.gapid.rpclib.binary.Encoder;
import com.google.gapid.rpclib.binary.Namespace;
import com.google.gapid.rpclib.schema.Array;
import com.google.gapid.rpclib.schema.Entity;
import com.google.gapid.rpclib.schema.Field;
import com.google.gapid.rpclib.schema.Method;
import com.google.gapid.rpclib.schema.Primitive;

import java.io.IOException;

final class Id implements BinaryObject {
    //<<<Start:Java.ClassBody:1>>>
    private BinaryID mValue;

    // Constructs a default-initialized {@link Id}.
    public Id() {}


    public BinaryID getValue() {
        return mValue;
    }

    public Id setValue(BinaryID v) {
        mValue = v;
        return this;
    }

    @Override
    public BinaryClass klass() { return Klass.INSTANCE; }


    private static final Entity ENTITY = new Entity("any", "id_", "", "");

    static {
        ENTITY.setFields(new Field[]{
            new Field("Value", new Array("id.ID", new Primitive("byte", Method.Uint8), 20)),
        });
        Namespace.register(Klass.INSTANCE);
    }
    public static void register() {}
    //<<<End:Java.ClassBody:1>>>
    public enum Klass implements BinaryClass {
        //<<<Start:Java.KlassBody:2>>>
        INSTANCE;

        @Override
        public Entity entity() { return ENTITY; }

        @Override
        public BinaryObject create() { return new Id(); }

        @Override
        public void encode(Encoder e, BinaryObject obj) throws IOException {
            Id o = (Id)obj;
            o.mValue.write(e);

        }

        @Override
        public void decode(Decoder d, BinaryObject obj) throws IOException {
            Id o = (Id)obj;
            o.mValue = new BinaryID(d);
        }
        //<<<End:Java.KlassBody:2>>>
    }
}
