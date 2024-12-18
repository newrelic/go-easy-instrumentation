// codegen is a library that creates, decorates or modifies DST objects in specific repeatable ways.
// This library is a common place for logic around how new nodes in a DST tree get created, as well
// as how to handle the whitespace and comments related to those nodes and the elements around them.
// Any function that creates a new node for insertion into the tree should be added here. When
// implementing functions for this library, the following rules should apply:
//
// 1. Any DST objects (expressions, statements, nodes, etc.) that are consumed as inputs should be
// defensively cloned before returning them as part of an output. There is a small execution cost to
// this, but if an object is duplicated anywhere in the tree, a runtime panic will occur.
// 2. Please add a comment header about what the output of your function is and what it does. All
// exported functions MUST be documented in way that is compatible with `godoc`.
// 3. Unit tests can be basic since the generated objects are going to be covered in many ways by
// the end to end tests. However, if a node gets returns that is invalid, it will fail to reneder and
// may result in a panic, which is not an acceptable outcome. A test to verify that the output is what
// we expect is a good safegard.
package codegen
