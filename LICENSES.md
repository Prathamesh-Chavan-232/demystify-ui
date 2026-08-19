# Third-party licenses

## The distinction that matters

Emitting a **theme object** shaped for MUI / Ant / Chakra is configuration data.
It carries no upstream copyright and needs no notice.

Shipping **component source** derived from shadcn, Radix, or Ant is different:
all three are MIT, and MIT requires the copyright notice and permission notice
be retained in redistributed copies. "Convert my shadcn button to CSS modules"
distributes derivative MIT code, so the notice travels with it.

Any codemod path that emits derived component source MUST also emit the
corresponding notice. See `packages/codemod`.

## Dependencies

| Project                            | License                                        | Used by                      | Notice required on redistribution |
| ---------------------------------- | ---------------------------------------------- | ---------------------------- | --------------------------------- |
| Ark UI / Zag.js                    | MIT                                            | `packages/ui`                | Yes — component source is shipped |
| Radix (primitives, themes, colors) | MIT                                            | `packages/adapters`          | Only if source is emitted         |
| shadcn/ui                          | MIT                                            | `packages/codemod`, registry | Yes — source is emitted           |
| culori                             | MIT                                            | `packages/engine`            | No — not redistributed            |
| apca-w3                            | W3C / Apache-2.0 (verify before first release) | `packages/engine`            | Verify                            |
| ts-morph, recast                   | MIT                                            | `packages/codemod`           | No — build-time only              |
| Ant Design                         | MIT                                            | `packages/adapters`          | Only if source is emitted         |
| Bootstrap                          | MIT                                            | `packages/adapters`          | Only if source is emitted         |

> `apca-w3` licensing needs confirming before the first public release; the APCA
> materials have carried more than one license across versions.
