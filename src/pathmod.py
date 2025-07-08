import os

def sep_strip(path):
  """Strips the leading path separator from a path, if present."""
  if not path:
    return path

  while path.startswith(os.sep):
    path = path[1:]
  return path
