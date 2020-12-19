The project is a static site generator. 

This project is in development. Features can change at any time. There is no documentation.

=== Notes ===

For pages, the URL path is computed from the file path using the following rules:

- prefix/index.html -> prefix/
- prefix/name.index.html  -> prefix/name
- prefix/name.html -> prefix/name/

The action <% set page="path" %> overrides the mapping above, but does not
change the path used for page queries.
