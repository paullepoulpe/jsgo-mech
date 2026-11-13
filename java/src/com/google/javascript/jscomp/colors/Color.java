/*
 * Copyright 2020 The Closure Compiler Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.google.javascript.jscomp.colors;

import static com.google.common.base.Preconditions.checkState;
import static java.util.Objects.requireNonNull;

import com.google.common.collect.ImmutableSet;
import com.google.common.collect.Iterables;
import java.util.LinkedHashSet;
import java.util.Set;
import org.jspecify.annotations.Nullable;

/** A simplified version of a Closure or TS type for use by optimizations */
public final class Color {
  private final ColorId id;
  private final ImmutableSet<Color> prototypes;
  private final ImmutableSet<Color> instanceColors;
  private final boolean invalidating;
  private final boolean propertiesKeepOriginalName;
  private final boolean constructor;
  private final ImmutableSet<String> ownProperties;
  private final @Nullable ColorId boxId;
  private final boolean closureAssert;
  private final ImmutableSet<Color> unionElements;
  private transient volatile Color memoizedSubtractNullOrVoid;

  private Color(
      ColorId id,
      ImmutableSet<Color> prototypes,
      ImmutableSet<Color> instanceColors,
      boolean invalidating,
      boolean propertiesKeepOriginalName,
      boolean constructor,
      ImmutableSet<String> ownProperties,
      @Nullable ColorId boxId,
      boolean closureAssert,
      ImmutableSet<Color> unionElements) {
    this.id = requireNonNull(id, "id");
    this.prototypes = requireNonNull(prototypes, "prototypes");
    this.instanceColors = requireNonNull(instanceColors, "instanceColors");
    this.invalidating = invalidating;
    this.propertiesKeepOriginalName = propertiesKeepOriginalName;
    this.constructor = constructor;
    this.ownProperties = requireNonNull(ownProperties, "ownProperties");
    this.boxId = boxId;
    this.closureAssert = closureAssert;
    this.unionElements = requireNonNull(unionElements, "unionElements");
  }

  @Override
  public String toString() {
    return "Color{"
        + "id=" + id + ", "
        + "prototypes=" + prototypes + ", "
        + "instanceColors=" + instanceColors + ", "
        + "invalidating=" + invalidating + ", "
        + "propertiesKeepOriginalName=" + propertiesKeepOriginalName + ", "
        + "constructor=" + constructor + ", "
        + "ownProperties=" + ownProperties + ", "
        + "boxId=" + boxId + ", "
        + "closureAssert=" + closureAssert + ", "
        + "unionElements=" + unionElements
        + "}";
  }

  @Override
  public boolean equals(@Nullable Object o) {
    if (o == this) {
      return true;
    }
    if (o instanceof Color) {
      Color that = (Color) o;
      return this.id.equals(that.getId())
          && this.prototypes.equals(that.getPrototypes())
          && this.instanceColors.equals(that.getInstanceColors())
          && this.invalidating == that.isInvalidating()
          && this.propertiesKeepOriginalName == that.getPropertiesKeepOriginalName()
          && this.constructor == that.isConstructor()
          && this.ownProperties.equals(that.getOwnProperties())
          && (this.boxId == null ? that.getBoxId() == null : this.boxId.equals(that.getBoxId()))
          && this.closureAssert == that.isClosureAssert()
          && this.unionElements.equals(that.getUnionElements());
    }
    return false;
  }

  @Override
  public int hashCode() {
    int h = 1;
    h *= 1000003;
    h ^= id.hashCode();
    h *= 1000003;
    h ^= prototypes.hashCode();
    h *= 1000003;
    h ^= instanceColors.hashCode();
    h *= 1000003;
    h ^= invalidating ? 1231 : 1237;
    h *= 1000003;
    h ^= propertiesKeepOriginalName ? 1231 : 1237;
    h *= 1000003;
    h ^= constructor ? 1231 : 1237;
    h *= 1000003;
    h ^= ownProperties.hashCode();
    h *= 1000003;
    h ^= (boxId == null) ? 0 : boxId.hashCode();
    h *= 1000003;
    h ^= closureAssert ? 1231 : 1237;
    h *= 1000003;
    h ^= unionElements.hashCode();
    return h;
  }

  public ColorId getId() {
    return id;
  }

  /** Given `function Foo() {}` or `class Foo {}`, color of Foo.prototype. null otherwise. */
  public ImmutableSet<Color> getPrototypes() {
    return prototypes;
  }

  public ImmutableSet<Color> getInstanceColors() {
    return instanceColors;
  }

  public boolean isInvalidating() {
    return invalidating;
  }

  public boolean getPropertiesKeepOriginalName() {
    return propertiesKeepOriginalName;
  }

  public boolean isConstructor() {
    return constructor;
  }

  /**
   * Property names 'declared' on an object (as opposed to being conceptually inherited from some
   * supertype).
   */
  public ImmutableSet<String> getOwnProperties() {
    return ownProperties;
  }

  public @Nullable ColorId getBoxId() {
    return boxId;
  }

  /**
   * Whether this type is some Closure assertion function removable by Closure-specific
   * optimizations.
   */
  public boolean isClosureAssert() {
    return closureAssert;
  }

  public ImmutableSet<Color> getUnionElements() {
    return unionElements;
  }

  public static Builder singleBuilder() {
    return new Builder()
        .setClosureAssert(false)
        .setConstructor(false)
        .setInstanceColors(ImmutableSet.of())
        .setInvalidating(false)
        .setOwnProperties(ImmutableSet.of())
        .setPropertiesKeepOriginalName(false)
        .setPrototypes(ImmutableSet.of())
        .setUnionElements(ImmutableSet.of());
  }

  public static Color createUnion(Set<Color> elements) {
    switch (elements.size()) {
      case 0 -> throw new IllegalStateException();
      case 1 -> {
        return Iterables.getOnlyElement(elements);
      }
      default -> {}
    }

    ImmutableSet.Builder<Color> instanceColors = ImmutableSet.builder();
    ImmutableSet.Builder<Color> prototypes = ImmutableSet.builder();
    ImmutableSet.Builder<Color> newElements = ImmutableSet.builder();
    ImmutableSet.Builder<ColorId> ids = ImmutableSet.builder();
    ImmutableSet.Builder<String> ownProperties = ImmutableSet.builder();
    boolean isClosureAssert = true;
    boolean isConstructor = true;
    boolean isInvalidating = false;
    boolean propertiesKeepOriginalName = false;

    for (Color element : elements) {
      if (element.isUnion()) {
        for (Color nestedElement : element.getUnionElements()) {
          newElements.add(nestedElement);
          ids.add(nestedElement.getId());
        }
      } else {
        newElements.add(element);
        ids.add(element.getId());
      }

      instanceColors.addAll(element.getInstanceColors());
      isClosureAssert &= element.isClosureAssert();
      isConstructor &= element.isConstructor();
      isInvalidating |= element.isInvalidating();
      ownProperties.addAll(element.getOwnProperties());
      propertiesKeepOriginalName |= element.getPropertiesKeepOriginalName();
      prototypes.addAll(element.getPrototypes());
    }

    return new Builder()
        .setClosureAssert(isClosureAssert)
        .setConstructor(isConstructor)
        .setId(ColorId.union(ids.build()))
        .setInstanceColors(instanceColors.build())
        .setInvalidating(isInvalidating)
        .setOwnProperties(ownProperties.build())
        .setPropertiesKeepOriginalName(propertiesKeepOriginalName)
        .setPrototypes(prototypes.build())
        .setUnionElements(newElements.build())
        .buildUnion();
  }

  /**
   * Whether this corresponds to a single JavaScript primitive like number or symbol.
   *
   * <p>Note that the boxed versions of primitives (String, Number, etc.) are /not/ considered
   * "primitive" by this method.
   */
  public final boolean isPrimitive() {
    /*
     * Avoid the design headache about whether unions are primitives. The union *color* isn't
     * primitive, but the *value* held by a union reference may be.
     */
    checkState(!this.isUnion(), this);
    return StandardColors.PRIMITIVE_COLORS.containsKey(this.getId());
  }

  public final boolean isUnion() {
    // Single element sets are banned in the builder.
    return !this.getUnionElements().isEmpty();
  }

  public Color subtractNullOrVoid() {
    if (memoizedSubtractNullOrVoid == null) {
      synchronized (this) {
        if (memoizedSubtractNullOrVoid == null) {
          memoizedSubtractNullOrVoid = requireNonNull(subtractNullOrVoidUncached());
        }
      }
    }
    return memoizedSubtractNullOrVoid;
  }

  private Color subtractNullOrVoidUncached() {
    /* Avoid defining NULL_OR_VOID.subtract(NULL_OR_VOID) */
    checkState(this.isUnion());

    if (!this.getUnionElements().contains(StandardColors.NULL_OR_VOID)) {
      return this;
    }

    LinkedHashSet<Color> elements = new LinkedHashSet<>(this.getUnionElements());
    elements.remove(StandardColors.NULL_OR_VOID);
    return createUnion(elements);
  }

  /**
   * Builder for a singleton color. Should be passed to {@link
   * Color#createSingleton(SingletonColorFields)} after building and before using
   */
  public static final class Builder {
    private @Nullable ColorId id;
    private @Nullable ImmutableSet<Color> prototypes;
    private @Nullable ImmutableSet<Color> instanceColors;
    private boolean invalidating;
    private boolean propertiesKeepOriginalName;
    private boolean constructor;
    private @Nullable ImmutableSet<String> ownProperties;
    private @Nullable ColorId boxId;
    private boolean closureAssert;
    private @Nullable ImmutableSet<Color> unionElements;
    private byte set0;

    Builder() {
    }


    public Builder setId(ColorId x) {
      if (x == null) {
        throw new NullPointerException("Null id");
      }
      this.id = x;
      return this;
    }

    public Builder setInvalidating(boolean x) {
      this.invalidating = x;
      set0 |= (byte) 1;
      return this;
    }

    public Builder setPropertiesKeepOriginalName(boolean x) {
      this.propertiesKeepOriginalName = x;
      set0 |= (byte) 2;
      return this;
    }

    public Builder setConstructor(boolean x) {
      this.constructor = x;
      set0 |= (byte) 4;
      return this;
    }

    public Builder setOwnProperties(ImmutableSet<String> x) {
      if (x == null) {
        throw new NullPointerException("Null ownProperties");
      }
      this.ownProperties = x;
      return this;
    }

    public Builder setClosureAssert(boolean x) {
      this.closureAssert = x;
      set0 |= (byte) 8;
      return this;
    }

    public Builder setInstanceColors(ImmutableSet<Color> x) {
      if (x == null) {
        throw new NullPointerException("Null instanceColors");
      }
      this.instanceColors = x;
      return this;
    }

    public Builder setPrototypes(ImmutableSet<Color> x) {
      if (x == null) {
        throw new NullPointerException("Null prototypes");
      }
      this.prototypes = x;
      return this;
    }

    Builder setUnionElements(ImmutableSet<Color> x) {
      if (x == null) {
        throw new NullPointerException("Null unionElements");
      }
      this.unionElements = x;
      return this;
    }

    Builder setBoxId(@Nullable ColorId x) {
      this.boxId = x;
      return this;
    }

    public Builder setPrototype(Color x) {
      return this.setPrototypes((x == null) ? ImmutableSet.of() : ImmutableSet.of(x));
    }

    public Builder setInstanceColor(Color x) {
      return this.setInstanceColors((x == null) ? ImmutableSet.of() : ImmutableSet.of(x));
    }

    Color buildInternal() {
      if (set0 != 0xf
          || this.id == null
          || this.prototypes == null
          || this.instanceColors == null
          || this.ownProperties == null
          || this.unionElements == null) {
        StringBuilder missing = new StringBuilder();
        if (this.id == null) {
          missing.append(" id");
        }
        if (this.prototypes == null) {
          missing.append(" prototypes");
        }
        if (this.instanceColors == null) {
          missing.append(" instanceColors");
        }
        if ((set0 & 1) == 0) {
          missing.append(" invalidating");
        }
        if ((set0 & 2) == 0) {
          missing.append(" propertiesKeepOriginalName");
        }
        if ((set0 & 4) == 0) {
          missing.append(" constructor");
        }
        if (this.ownProperties == null) {
          missing.append(" ownProperties");
        }
        if ((set0 & 8) == 0) {
          missing.append(" closureAssert");
        }
        if (this.unionElements == null) {
          missing.append(" unionElements");
        }
        throw new IllegalStateException("Missing required properties:" + missing);
      }
      return new Color(
          this.id,
          this.prototypes,
          this.instanceColors,
          this.invalidating,
          this.propertiesKeepOriginalName,
          this.constructor,
          this.ownProperties,
          this.boxId,
          this.closureAssert,
          this.unionElements);
    }

    public final Color build() {
      Color result = this.buildInternal();
      checkState(result.getUnionElements().isEmpty(), result);
      checkState(result.getBoxId() == null, result);
      checkState(!StandardColors.AXIOMATIC_COLORS.containsKey(result.getId()), result);
      return result;
    }

    private final Color buildUnion() {
      Color result = this.buildInternal();
      checkState(result.getUnionElements().size() > 1, result);
      checkState(result.getBoxId() == null, result);
      return result;
    }

    final Color buildAxiomatic() {
      checkState(StandardColors.AXIOMATIC_COLORS == null, "StandardColors are all defined");

      Color result = this.buildInternal();
      checkState(result.getUnionElements().isEmpty(), result);
      return result;
    }
  }
}
