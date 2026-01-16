all
# Exclude MD022 - Headers should be surrounded by blank lines. The kramdown
# "class magic" (like {: .no_toc}) needs to be directly below the heading line.
exclude_rule 'MD022'
# Exclude MD041 - First line in file should be a top level header
exclude_rule 'MD041'
rule 'MD013', :tables => false
rule 'MD007', :indent => 2
rule 'MD013', :ignore_code_blocks => true
rule 'MD024', :allow_different_nesting => true
# MD056 - Inconsistent number of columns in table
# docs/deployment/helm.md:98
exclude_rule 'MD056'
# MD032 - Lists should be surrounded by blank lines
# Auto-generated API docs (crd-ref-docs) have _Appears in:_ followed by list
exclude_rule 'MD032'
# MD033 - Inline HTML
# Auto-generated API docs use <br /> for line breaks in table cells
exclude_rule 'MD033'
# MD012 - Multiple consecutive blank lines
# Auto-generated API docs may have extra blank lines
exclude_rule 'MD012'
# MD013 - Line length
# Auto-generated API docs have long type signatures that cannot be wrapped
exclude_rule 'MD013'
