all
# Exclude MD041 - First line in file should be a top level header
exclude_rule 'MD041'
rule 'MD013', :tables => false
rule 'MD013', :code_blocks => false
rule 'MD024', :allow_different_nesting => true
