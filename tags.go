// Tags is a helper description for a structure that stores a map of
// attributes and values for a given key.
//
// Example:
// Tags["1.2.3.4"]["dst_hostname"] = "localhost"
// Tags["1.2.3.4"]["dst_cluster"] = "mycluster"
package llama

// TODO(dmar): This is cool and all, but as is, it just plays weird. You need
//       to make it like a map, but that only does the outside. Try to do a
//       layer deep and it panics. And even those it's just a string map, it
//       won't actually accept any string map. So it's just being more strict.

type Tags map[string]string
type TagSet map[string]Tags
