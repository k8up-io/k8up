#!/usr/bin/env python3

# This script alerts of orphan files in the modules/ROOT/pages directory
# that are not referenced in either ./k8up.adoc or modules/ROOT/nav.adoc.

from os import listdir
from os.path import isfile, join
import re

# Opens the input file referenced by its filename, and uses the
# regular expression to filter all included files. It then
# iterates over the list and appends an error if a file in
# the directory is not explicitly referenced in the input.
def check(input, regex):
    with open(input, 'r') as file:
        contents = file.read()
    matches = re.findall(regex, contents)
    for f in adoc_files:
        if f not in matches:
            errors.append('File "{entry}" not in {input}'.format(entry = f, input = input))

# Global variables
pages_dir = 'modules/ROOT/pages'
adoc_files = [f for f in listdir(pages_dir) if isfile(join(pages_dir, f))]
errors = []

# TODO adapt to special nav structure with partials

# Perform checks
check('k8up.adoc',             r'include::modules/ROOT/pages/(.+)\[\]')
check('modules/ROOT/nav.adoc', r'xref:(.+)\[')

# Exit with error if some file is orphan
if len(errors) > 0:
    for e in errors:
        print(e)
    exit(1)

# All is well, bye bye
print('No orphan files in either k8up.adoc or modules/ROOT/nav.adoc')
exit(0)

