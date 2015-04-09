package filter

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

const end_symbol rune = 4

/* The rule types inferred from the grammar are below. */
type pegRule uint8

const (
	ruleUnknown pegRule = iota
	ruleFILTER
	ruleExpr
	ruleorExpr
	rulefactor
	ruleandExpr
	ruleprimary
	ruleagent
	rulelabel
	ruleregexp
	rulechar
	rulews
	rulewsp
	ruleAction0
	ruleAction1
	ruleAction2
	rulePegText
	ruleAction3
	ruleAction4

	rulePre_
	rule_In_
	rule_Suf
)

var rul3s = [...]string{
	"Unknown",
	"FILTER",
	"Expr",
	"orExpr",
	"factor",
	"andExpr",
	"primary",
	"agent",
	"label",
	"regexp",
	"char",
	"ws",
	"wsp",
	"Action0",
	"Action1",
	"Action2",
	"PegText",
	"Action3",
	"Action4",

	"Pre_",
	"_In_",
	"_Suf",
}

type tokenTree interface {
	Print()
	PrintSyntax()
	PrintSyntaxTree(buffer string)
	Add(rule pegRule, begin, end, next, depth int)
	Expand(index int) tokenTree
	Tokens() <-chan token32
	AST() *node32
	Error() []token32
	trim(length int)
}

type node32 struct {
	token32
	up, next *node32
}

func (node *node32) print(depth int, buffer string) {
	for node != nil {
		for c := 0; c < depth; c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[node.pegRule], strconv.Quote(string(([]rune(buffer)[node.begin:node.end]))))
		if node.up != nil {
			node.up.print(depth+1, buffer)
		}
		node = node.next
	}
}

func (ast *node32) Print(buffer string) {
	ast.print(0, buffer)
}

type element struct {
	node *node32
	down *element
}

/* ${@} bit structure for abstract syntax tree */
type token16 struct {
	pegRule
	begin, end, next int16
}

func (t *token16) isZero() bool {
	return t.pegRule == ruleUnknown && t.begin == 0 && t.end == 0 && t.next == 0
}

func (t *token16) isParentOf(u token16) bool {
	return t.begin <= u.begin && t.end >= u.end && t.next > u.next
}

func (t *token16) getToken32() token32 {
	return token32{pegRule: t.pegRule, begin: int32(t.begin), end: int32(t.end), next: int32(t.next)}
}

func (t *token16) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v %v", rul3s[t.pegRule], t.begin, t.end, t.next)
}

type tokens16 struct {
	tree    []token16
	ordered [][]token16
}

func (t *tokens16) trim(length int) {
	t.tree = t.tree[0:length]
}

func (t *tokens16) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens16) Order() [][]token16 {
	if t.ordered != nil {
		return t.ordered
	}

	depths := make([]int16, 1, math.MaxInt16)
	for i, token := range t.tree {
		if token.pegRule == ruleUnknown {
			t.tree = t.tree[:i]
			break
		}
		depth := int(token.next)
		if length := len(depths); depth >= length {
			depths = depths[:depth+1]
		}
		depths[depth]++
	}
	depths = append(depths, 0)

	ordered, pool := make([][]token16, len(depths)), make([]token16, len(t.tree)+len(depths))
	for i, depth := range depths {
		depth++
		ordered[i], pool, depths[i] = pool[:depth], pool[depth:], 0
	}

	for i, token := range t.tree {
		depth := token.next
		token.next = int16(i)
		ordered[depth][depths[depth]] = token
		depths[depth]++
	}
	t.ordered = ordered
	return ordered
}

type state16 struct {
	token16
	depths []int16
	leaf   bool
}

func (t *tokens16) AST() *node32 {
	tokens := t.Tokens()
	stack := &element{node: &node32{token32: <-tokens}}
	for token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	return stack.node
}

func (t *tokens16) PreOrder() (<-chan state16, [][]token16) {
	s, ordered := make(chan state16, 6), t.Order()
	go func() {
		var states [8]state16
		for i, _ := range states {
			states[i].depths = make([]int16, len(ordered))
		}
		depths, state, depth := make([]int16, len(ordered)), 0, 1
		write := func(t token16, leaf bool) {
			S := states[state]
			state, S.pegRule, S.begin, S.end, S.next, S.leaf = (state+1)%8, t.pegRule, t.begin, t.end, int16(depth), leaf
			copy(S.depths, depths)
			s <- S
		}

		states[state].token16 = ordered[0][0]
		depths[0]++
		state++
		a, b := ordered[depth-1][depths[depth-1]-1], ordered[depth][depths[depth]]
	depthFirstSearch:
		for {
			for {
				if i := depths[depth]; i > 0 {
					if c, j := ordered[depth][i-1], depths[depth-1]; a.isParentOf(c) &&
						(j < 2 || !ordered[depth-1][j-2].isParentOf(c)) {
						if c.end != b.begin {
							write(token16{pegRule: rule_In_, begin: c.end, end: b.begin}, true)
						}
						break
					}
				}

				if a.begin < b.begin {
					write(token16{pegRule: rulePre_, begin: a.begin, end: b.begin}, true)
				}
				break
			}

			next := depth + 1
			if c := ordered[next][depths[next]]; c.pegRule != ruleUnknown && b.isParentOf(c) {
				write(b, false)
				depths[depth]++
				depth, a, b = next, b, c
				continue
			}

			write(b, true)
			depths[depth]++
			c, parent := ordered[depth][depths[depth]], true
			for {
				if c.pegRule != ruleUnknown && a.isParentOf(c) {
					b = c
					continue depthFirstSearch
				} else if parent && b.end != a.end {
					write(token16{pegRule: rule_Suf, begin: b.end, end: a.end}, true)
				}

				depth--
				if depth > 0 {
					a, b, c = ordered[depth-1][depths[depth-1]-1], a, ordered[depth][depths[depth]]
					parent = a.isParentOf(b)
					continue
				}

				break depthFirstSearch
			}
		}

		close(s)
	}()
	return s, ordered
}

func (t *tokens16) PrintSyntax() {
	tokens, ordered := t.PreOrder()
	max := -1
	for token := range tokens {
		if !token.leaf {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[36m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[36m%v\x1B[m\n", rul3s[token.pegRule])
		} else if token.begin == token.end {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[31m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[31m%v\x1B[m\n", rul3s[token.pegRule])
		} else {
			for c, end := token.begin, token.end; c < end; c++ {
				if i := int(c); max+1 < i {
					for j := max; j < i; j++ {
						fmt.Printf("skip %v %v\n", j, token.String())
					}
					max = i
				} else if i := int(c); i <= max {
					for j := i; j <= max; j++ {
						fmt.Printf("dupe %v %v\n", j, token.String())
					}
				} else {
					max = int(c)
				}
				fmt.Printf("%v", c)
				for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
					fmt.Printf(" \x1B[34m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
				}
				fmt.Printf(" \x1B[34m%v\x1B[m\n", rul3s[token.pegRule])
			}
			fmt.Printf("\n")
		}
	}
}

func (t *tokens16) PrintSyntaxTree(buffer string) {
	tokens, _ := t.PreOrder()
	for token := range tokens {
		for c := 0; c < int(token.next); c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[token.pegRule], strconv.Quote(string(([]rune(buffer)[token.begin:token.end]))))
	}
}

func (t *tokens16) Add(rule pegRule, begin, end, depth, index int) {
	t.tree[index] = token16{pegRule: rule, begin: int16(begin), end: int16(end), next: int16(depth)}
}

func (t *tokens16) Tokens() <-chan token32 {
	s := make(chan token32, 16)
	go func() {
		for _, v := range t.tree {
			s <- v.getToken32()
		}
		close(s)
	}()
	return s
}

func (t *tokens16) Error() []token32 {
	ordered := t.Order()
	length := len(ordered)
	tokens, length := make([]token32, length), length-1
	for i, _ := range tokens {
		o := ordered[length-i]
		if len(o) > 1 {
			tokens[i] = o[len(o)-2].getToken32()
		}
	}
	return tokens
}

/* ${@} bit structure for abstract syntax tree */
type token32 struct {
	pegRule
	begin, end, next int32
}

func (t *token32) isZero() bool {
	return t.pegRule == ruleUnknown && t.begin == 0 && t.end == 0 && t.next == 0
}

func (t *token32) isParentOf(u token32) bool {
	return t.begin <= u.begin && t.end >= u.end && t.next > u.next
}

func (t *token32) getToken32() token32 {
	return token32{pegRule: t.pegRule, begin: int32(t.begin), end: int32(t.end), next: int32(t.next)}
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v %v", rul3s[t.pegRule], t.begin, t.end, t.next)
}

type tokens32 struct {
	tree    []token32
	ordered [][]token32
}

func (t *tokens32) trim(length int) {
	t.tree = t.tree[0:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) Order() [][]token32 {
	if t.ordered != nil {
		return t.ordered
	}

	depths := make([]int32, 1, math.MaxInt16)
	for i, token := range t.tree {
		if token.pegRule == ruleUnknown {
			t.tree = t.tree[:i]
			break
		}
		depth := int(token.next)
		if length := len(depths); depth >= length {
			depths = depths[:depth+1]
		}
		depths[depth]++
	}
	depths = append(depths, 0)

	ordered, pool := make([][]token32, len(depths)), make([]token32, len(t.tree)+len(depths))
	for i, depth := range depths {
		depth++
		ordered[i], pool, depths[i] = pool[:depth], pool[depth:], 0
	}

	for i, token := range t.tree {
		depth := token.next
		token.next = int32(i)
		ordered[depth][depths[depth]] = token
		depths[depth]++
	}
	t.ordered = ordered
	return ordered
}

type state32 struct {
	token32
	depths []int32
	leaf   bool
}

func (t *tokens32) AST() *node32 {
	tokens := t.Tokens()
	stack := &element{node: &node32{token32: <-tokens}}
	for token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	return stack.node
}

func (t *tokens32) PreOrder() (<-chan state32, [][]token32) {
	s, ordered := make(chan state32, 6), t.Order()
	go func() {
		var states [8]state32
		for i, _ := range states {
			states[i].depths = make([]int32, len(ordered))
		}
		depths, state, depth := make([]int32, len(ordered)), 0, 1
		write := func(t token32, leaf bool) {
			S := states[state]
			state, S.pegRule, S.begin, S.end, S.next, S.leaf = (state+1)%8, t.pegRule, t.begin, t.end, int32(depth), leaf
			copy(S.depths, depths)
			s <- S
		}

		states[state].token32 = ordered[0][0]
		depths[0]++
		state++
		a, b := ordered[depth-1][depths[depth-1]-1], ordered[depth][depths[depth]]
	depthFirstSearch:
		for {
			for {
				if i := depths[depth]; i > 0 {
					if c, j := ordered[depth][i-1], depths[depth-1]; a.isParentOf(c) &&
						(j < 2 || !ordered[depth-1][j-2].isParentOf(c)) {
						if c.end != b.begin {
							write(token32{pegRule: rule_In_, begin: c.end, end: b.begin}, true)
						}
						break
					}
				}

				if a.begin < b.begin {
					write(token32{pegRule: rulePre_, begin: a.begin, end: b.begin}, true)
				}
				break
			}

			next := depth + 1
			if c := ordered[next][depths[next]]; c.pegRule != ruleUnknown && b.isParentOf(c) {
				write(b, false)
				depths[depth]++
				depth, a, b = next, b, c
				continue
			}

			write(b, true)
			depths[depth]++
			c, parent := ordered[depth][depths[depth]], true
			for {
				if c.pegRule != ruleUnknown && a.isParentOf(c) {
					b = c
					continue depthFirstSearch
				} else if parent && b.end != a.end {
					write(token32{pegRule: rule_Suf, begin: b.end, end: a.end}, true)
				}

				depth--
				if depth > 0 {
					a, b, c = ordered[depth-1][depths[depth-1]-1], a, ordered[depth][depths[depth]]
					parent = a.isParentOf(b)
					continue
				}

				break depthFirstSearch
			}
		}

		close(s)
	}()
	return s, ordered
}

func (t *tokens32) PrintSyntax() {
	tokens, ordered := t.PreOrder()
	max := -1
	for token := range tokens {
		if !token.leaf {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[36m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[36m%v\x1B[m\n", rul3s[token.pegRule])
		} else if token.begin == token.end {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[31m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[31m%v\x1B[m\n", rul3s[token.pegRule])
		} else {
			for c, end := token.begin, token.end; c < end; c++ {
				if i := int(c); max+1 < i {
					for j := max; j < i; j++ {
						fmt.Printf("skip %v %v\n", j, token.String())
					}
					max = i
				} else if i := int(c); i <= max {
					for j := i; j <= max; j++ {
						fmt.Printf("dupe %v %v\n", j, token.String())
					}
				} else {
					max = int(c)
				}
				fmt.Printf("%v", c)
				for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
					fmt.Printf(" \x1B[34m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
				}
				fmt.Printf(" \x1B[34m%v\x1B[m\n", rul3s[token.pegRule])
			}
			fmt.Printf("\n")
		}
	}
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	tokens, _ := t.PreOrder()
	for token := range tokens {
		for c := 0; c < int(token.next); c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[token.pegRule], strconv.Quote(string(([]rune(buffer)[token.begin:token.end]))))
	}
}

func (t *tokens32) Add(rule pegRule, begin, end, depth, index int) {
	t.tree[index] = token32{pegRule: rule, begin: int32(begin), end: int32(end), next: int32(depth)}
}

func (t *tokens32) Tokens() <-chan token32 {
	s := make(chan token32, 16)
	go func() {
		for _, v := range t.tree {
			s <- v.getToken32()
		}
		close(s)
	}()
	return s
}

func (t *tokens32) Error() []token32 {
	ordered := t.Order()
	length := len(ordered)
	tokens, length := make([]token32, length), length-1
	for i, _ := range tokens {
		o := ordered[length-i]
		if len(o) > 1 {
			tokens[i] = o[len(o)-2].getToken32()
		}
	}
	return tokens
}

func (t *tokens16) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		for i, v := range tree {
			expanded[i] = v.getToken32()
		}
		return &tokens32{tree: expanded}
	}
	return nil
}

func (t *tokens32) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	return nil
}

type parser struct {
	stack []Evaluator
	err   error

	Buffer string
	buffer []rune
	rules  [19]func() bool
	Parse  func(rule ...int) error
	Reset  func()
	tokenTree
}

type textPosition struct {
	line, symbol int
}

type textPositionMap map[int]textPosition

func translatePositions(buffer string, positions []int) textPositionMap {
	length, translations, j, line, symbol := len(positions), make(textPositionMap, len(positions)), 0, 1, 0
	sort.Ints(positions)

search:
	for i, c := range buffer[0:] {
		if c == '\n' {
			line, symbol = line+1, 0
		} else {
			symbol++
		}
		if i == positions[j] {
			translations[positions[j]] = textPosition{line, symbol}
			for j++; j < length; j++ {
				if i != positions[j] {
					continue search
				}
			}
			break search
		}
	}

	return translations
}

type parseError struct {
	p *parser
}

func (e *parseError) Error() string {
	tokens, error := e.p.tokenTree.Error(), "\n"
	positions, p := make([]int, 2*len(tokens)), 0
	for _, token := range tokens {
		positions[p], p = int(token.begin), p+1
		positions[p], p = int(token.end), p+1
	}
	translations := translatePositions(e.p.Buffer, positions)
	for _, token := range tokens {
		begin, end := int(token.begin), int(token.end)
		error += fmt.Sprintf("parse error near \x1B[34m%v\x1B[m (line %v symbol %v - line %v symbol %v):\n%v\n",
			rul3s[token.pegRule],
			translations[begin].line, translations[begin].symbol,
			translations[end].line, translations[end].symbol,
			/*strconv.Quote(*/ e.p.Buffer[begin:end] /*)*/)
	}

	return error
}

func (p *parser) PrintSyntaxTree() {
	p.tokenTree.PrintSyntaxTree(p.Buffer)
}

func (p *parser) Highlighter() {
	p.tokenTree.PrintSyntax()
}

func (p *parser) Execute() {
	buffer, begin, end := p.Buffer, 0, 0
	for token := range p.tokenTree.Tokens() {
		switch token.pegRule {

		case rulePegText:
			begin, end = int(token.begin), int(token.end)

		case ruleAction0:
			p.Or()
		case ruleAction1:
			p.And()
		case ruleAction2:
			p.Not()
		case ruleAction3:
			p.Agent(buffer[begin:end])
		case ruleAction4:
			p.Label(buffer[begin:end])

		}
	}
	_, _, _ = buffer, begin, end
}

func (p *parser) Init() {
	p.buffer = []rune(p.Buffer)
	if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != end_symbol {
		p.buffer = append(p.buffer, end_symbol)
	}

	var tree tokenTree = &tokens16{tree: make([]token16, math.MaxInt16)}
	position, depth, tokenIndex, buffer, _rules := 0, 0, 0, p.buffer, p.rules

	p.Parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokenTree = tree
		if matches {
			p.tokenTree.trim(tokenIndex)
			return nil
		}
		return &parseError{p}
	}

	p.Reset = func() {
		position, tokenIndex, depth = 0, 0, 0
	}

	add := func(rule pegRule, begin int) {
		if t := tree.Expand(tokenIndex); t != nil {
			tree = t
		}
		tree.Add(rule, begin, position, depth, tokenIndex)
		tokenIndex++
	}

	matchDot := func() bool {
		if buffer[position] != end_symbol {
			position++
			return true
		}
		return false
	}

	/*matchChar := func(c byte) bool {
		if buffer[position] == c {
			position++
			return true
		}
		return false
	}*/

	_rules = [...]func() bool{
		nil,
		/* 0 FILTER <- <(Expr !.)> */
		func() bool {
			position0, tokenIndex0, depth0 := position, tokenIndex, depth
			{
				position1 := position
				depth++
				if !_rules[ruleExpr]() {
					goto l0
				}
				{
					position2, tokenIndex2, depth2 := position, tokenIndex, depth
					if !matchDot() {
						goto l2
					}
					goto l0
				l2:
					position, tokenIndex, depth = position2, tokenIndex2, depth2
				}
				depth--
				add(ruleFILTER, position1)
			}
			return true
		l0:
			position, tokenIndex, depth = position0, tokenIndex0, depth0
			return false
		},
		/* 1 Expr <- <(factor orExpr*)> */
		func() bool {
			position3, tokenIndex3, depth3 := position, tokenIndex, depth
			{
				position4 := position
				depth++
				if !_rules[rulefactor]() {
					goto l3
				}
			l5:
				{
					position6, tokenIndex6, depth6 := position, tokenIndex, depth
					{
						position7 := position
						depth++
						if !_rules[rulewsp]() {
							goto l6
						}
						{
							position8, tokenIndex8, depth8 := position, tokenIndex, depth
							if buffer[position] != rune('o') {
								goto l9
							}
							position++
							goto l8
						l9:
							position, tokenIndex, depth = position8, tokenIndex8, depth8
							if buffer[position] != rune('O') {
								goto l6
							}
							position++
						}
					l8:
						{
							position10, tokenIndex10, depth10 := position, tokenIndex, depth
							if buffer[position] != rune('r') {
								goto l11
							}
							position++
							goto l10
						l11:
							position, tokenIndex, depth = position10, tokenIndex10, depth10
							if buffer[position] != rune('R') {
								goto l6
							}
							position++
						}
					l10:
						if !_rules[rulewsp]() {
							goto l6
						}
						if !_rules[rulefactor]() {
							goto l6
						}
						{
							add(ruleAction0, position)
						}
						depth--
						add(ruleorExpr, position7)
					}
					goto l5
				l6:
					position, tokenIndex, depth = position6, tokenIndex6, depth6
				}
				depth--
				add(ruleExpr, position4)
			}
			return true
		l3:
			position, tokenIndex, depth = position3, tokenIndex3, depth3
			return false
		},
		/* 2 orExpr <- <(wsp (('o' / 'O') ('r' / 'R')) wsp factor Action0)> */
		nil,
		/* 3 factor <- <(primary andExpr*)> */
		func() bool {
			position14, tokenIndex14, depth14 := position, tokenIndex, depth
			{
				position15 := position
				depth++
				if !_rules[ruleprimary]() {
					goto l14
				}
			l16:
				{
					position17, tokenIndex17, depth17 := position, tokenIndex, depth
					{
						position18 := position
						depth++
						if !_rules[rulewsp]() {
							goto l17
						}
						{
							position19, tokenIndex19, depth19 := position, tokenIndex, depth
							if buffer[position] != rune('a') {
								goto l20
							}
							position++
							goto l19
						l20:
							position, tokenIndex, depth = position19, tokenIndex19, depth19
							if buffer[position] != rune('A') {
								goto l17
							}
							position++
						}
					l19:
						{
							position21, tokenIndex21, depth21 := position, tokenIndex, depth
							if buffer[position] != rune('n') {
								goto l22
							}
							position++
							goto l21
						l22:
							position, tokenIndex, depth = position21, tokenIndex21, depth21
							if buffer[position] != rune('N') {
								goto l17
							}
							position++
						}
					l21:
						{
							position23, tokenIndex23, depth23 := position, tokenIndex, depth
							if buffer[position] != rune('d') {
								goto l24
							}
							position++
							goto l23
						l24:
							position, tokenIndex, depth = position23, tokenIndex23, depth23
							if buffer[position] != rune('D') {
								goto l17
							}
							position++
						}
					l23:
						if !_rules[rulewsp]() {
							goto l17
						}
						if !_rules[ruleprimary]() {
							goto l17
						}
						{
							add(ruleAction1, position)
						}
						depth--
						add(ruleandExpr, position18)
					}
					goto l16
				l17:
					position, tokenIndex, depth = position17, tokenIndex17, depth17
				}
				depth--
				add(rulefactor, position15)
			}
			return true
		l14:
			position, tokenIndex, depth = position14, tokenIndex14, depth14
			return false
		},
		/* 4 andExpr <- <(wsp (('a' / 'A') ('n' / 'N') ('d' / 'D')) wsp primary Action1)> */
		nil,
		/* 5 primary <- <((&('L' | 'l') label) | (&('A' | 'a') agent) | (&('(') ('(' ws Expr ws ')')) | (&('N' | 'n') (('n' / 'N') ('o' / 'O') ('t' / 'T') wsp primary Action2)))> */
		func() bool {
			position27, tokenIndex27, depth27 := position, tokenIndex, depth
			{
				position28 := position
				depth++
				{
					switch buffer[position] {
					case 'L', 'l':
						{
							position30 := position
							depth++
							{
								position31, tokenIndex31, depth31 := position, tokenIndex, depth
								if buffer[position] != rune('l') {
									goto l32
								}
								position++
								goto l31
							l32:
								position, tokenIndex, depth = position31, tokenIndex31, depth31
								if buffer[position] != rune('L') {
									goto l27
								}
								position++
							}
						l31:
							if buffer[position] != rune('@') {
								goto l27
							}
							position++
							{
								position33 := position
								depth++
								if !_rules[rulechar]() {
									goto l27
								}
							l34:
								{
									position35, tokenIndex35, depth35 := position, tokenIndex, depth
									if !_rules[rulechar]() {
										goto l35
									}
									goto l34
								l35:
									position, tokenIndex, depth = position35, tokenIndex35, depth35
								}
								if buffer[position] != rune(':') {
									goto l27
								}
								position++
								if !_rules[rulechar]() {
									goto l27
								}
							l36:
								{
									position37, tokenIndex37, depth37 := position, tokenIndex, depth
									if !_rules[rulechar]() {
										goto l37
									}
									goto l36
								l37:
									position, tokenIndex, depth = position37, tokenIndex37, depth37
								}
								depth--
								add(rulePegText, position33)
							}
							{
								add(ruleAction4, position)
							}
							depth--
							add(rulelabel, position30)
						}
						break
					case 'A', 'a':
						{
							position39 := position
							depth++
							{
								position40, tokenIndex40, depth40 := position, tokenIndex, depth
								if buffer[position] != rune('a') {
									goto l41
								}
								position++
								goto l40
							l41:
								position, tokenIndex, depth = position40, tokenIndex40, depth40
								if buffer[position] != rune('A') {
									goto l27
								}
								position++
							}
						l40:
							if buffer[position] != rune('@') {
								goto l27
							}
							position++
							{
								position42 := position
								depth++
								if !_rules[ruleregexp]() {
									goto l27
								}
								depth--
								add(rulePegText, position42)
							}
							{
								add(ruleAction3, position)
							}
							depth--
							add(ruleagent, position39)
						}
						break
					case '(':
						if buffer[position] != rune('(') {
							goto l27
						}
						position++
						if !_rules[rulews]() {
							goto l27
						}
						if !_rules[ruleExpr]() {
							goto l27
						}
						if !_rules[rulews]() {
							goto l27
						}
						if buffer[position] != rune(')') {
							goto l27
						}
						position++
						break
					default:
						{
							position44, tokenIndex44, depth44 := position, tokenIndex, depth
							if buffer[position] != rune('n') {
								goto l45
							}
							position++
							goto l44
						l45:
							position, tokenIndex, depth = position44, tokenIndex44, depth44
							if buffer[position] != rune('N') {
								goto l27
							}
							position++
						}
					l44:
						{
							position46, tokenIndex46, depth46 := position, tokenIndex, depth
							if buffer[position] != rune('o') {
								goto l47
							}
							position++
							goto l46
						l47:
							position, tokenIndex, depth = position46, tokenIndex46, depth46
							if buffer[position] != rune('O') {
								goto l27
							}
							position++
						}
					l46:
						{
							position48, tokenIndex48, depth48 := position, tokenIndex, depth
							if buffer[position] != rune('t') {
								goto l49
							}
							position++
							goto l48
						l49:
							position, tokenIndex, depth = position48, tokenIndex48, depth48
							if buffer[position] != rune('T') {
								goto l27
							}
							position++
						}
					l48:
						if !_rules[rulewsp]() {
							goto l27
						}
						if !_rules[ruleprimary]() {
							goto l27
						}
						{
							add(ruleAction2, position)
						}
						break
					}
				}

				depth--
				add(ruleprimary, position28)
			}
			return true
		l27:
			position, tokenIndex, depth = position27, tokenIndex27, depth27
			return false
		},
		/* 6 agent <- <(('a' / 'A') '@' <regexp> Action3)> */
		nil,
		/* 7 label <- <(('l' / 'L') '@' <(char+ ':' char+)> Action4)> */
		nil,
		/* 8 regexp <- <((char* '(' regexp ')' char*) / (char* '|' regexp*) / char+)> */
		func() bool {
			position53, tokenIndex53, depth53 := position, tokenIndex, depth
			{
				position54 := position
				depth++
				{
					position55, tokenIndex55, depth55 := position, tokenIndex, depth
				l57:
					{
						position58, tokenIndex58, depth58 := position, tokenIndex, depth
						if !_rules[rulechar]() {
							goto l58
						}
						goto l57
					l58:
						position, tokenIndex, depth = position58, tokenIndex58, depth58
					}
					if buffer[position] != rune('(') {
						goto l56
					}
					position++
					if !_rules[ruleregexp]() {
						goto l56
					}
					if buffer[position] != rune(')') {
						goto l56
					}
					position++
				l59:
					{
						position60, tokenIndex60, depth60 := position, tokenIndex, depth
						if !_rules[rulechar]() {
							goto l60
						}
						goto l59
					l60:
						position, tokenIndex, depth = position60, tokenIndex60, depth60
					}
					goto l55
				l56:
					position, tokenIndex, depth = position55, tokenIndex55, depth55
				l62:
					{
						position63, tokenIndex63, depth63 := position, tokenIndex, depth
						if !_rules[rulechar]() {
							goto l63
						}
						goto l62
					l63:
						position, tokenIndex, depth = position63, tokenIndex63, depth63
					}
					if buffer[position] != rune('|') {
						goto l61
					}
					position++
				l64:
					{
						position65, tokenIndex65, depth65 := position, tokenIndex, depth
						if !_rules[ruleregexp]() {
							goto l65
						}
						goto l64
					l65:
						position, tokenIndex, depth = position65, tokenIndex65, depth65
					}
					goto l55
				l61:
					position, tokenIndex, depth = position55, tokenIndex55, depth55
					if !_rules[rulechar]() {
						goto l53
					}
				l66:
					{
						position67, tokenIndex67, depth67 := position, tokenIndex, depth
						if !_rules[rulechar]() {
							goto l67
						}
						goto l66
					l67:
						position, tokenIndex, depth = position67, tokenIndex67, depth67
					}
				}
			l55:
				depth--
				add(ruleregexp, position54)
			}
			return true
		l53:
			position, tokenIndex, depth = position53, tokenIndex53, depth53
			return false
		},
		/* 9 char <- <(!((&(':') ':') | (&(')') ')') | (&('(') '(') | (&(' ') ' ')) .)> */
		func() bool {
			position68, tokenIndex68, depth68 := position, tokenIndex, depth
			{
				position69 := position
				depth++
				{
					position70, tokenIndex70, depth70 := position, tokenIndex, depth
					{
						switch buffer[position] {
						case ':':
							if buffer[position] != rune(':') {
								goto l70
							}
							position++
							break
						case ')':
							if buffer[position] != rune(')') {
								goto l70
							}
							position++
							break
						case '(':
							if buffer[position] != rune('(') {
								goto l70
							}
							position++
							break
						default:
							if buffer[position] != rune(' ') {
								goto l70
							}
							position++
							break
						}
					}

					goto l68
				l70:
					position, tokenIndex, depth = position70, tokenIndex70, depth70
				}
				if !matchDot() {
					goto l68
				}
				depth--
				add(rulechar, position69)
			}
			return true
		l68:
			position, tokenIndex, depth = position68, tokenIndex68, depth68
			return false
		},
		/* 10 ws <- <' '*> */
		func() bool {
			{
				position73 := position
				depth++
			l74:
				{
					position75, tokenIndex75, depth75 := position, tokenIndex, depth
					if buffer[position] != rune(' ') {
						goto l75
					}
					position++
					goto l74
				l75:
					position, tokenIndex, depth = position75, tokenIndex75, depth75
				}
				depth--
				add(rulews, position73)
			}
			return true
		},
		/* 11 wsp <- <' '+> */
		func() bool {
			position76, tokenIndex76, depth76 := position, tokenIndex, depth
			{
				position77 := position
				depth++
				if buffer[position] != rune(' ') {
					goto l76
				}
				position++
			l78:
				{
					position79, tokenIndex79, depth79 := position, tokenIndex, depth
					if buffer[position] != rune(' ') {
						goto l79
					}
					position++
					goto l78
				l79:
					position, tokenIndex, depth = position79, tokenIndex79, depth79
				}
				depth--
				add(rulewsp, position77)
			}
			return true
		l76:
			position, tokenIndex, depth = position76, tokenIndex76, depth76
			return false
		},
		/* 13 Action0 <- <{ p.Or() }> */
		nil,
		/* 14 Action1 <- <{ p.And() }> */
		nil,
		/* 15 Action2 <- <{ p.Not() }> */
		nil,
		nil,
		/* 17 Action3 <- <{ p.Agent(buffer[begin:end]) }> */
		nil,
		/* 18 Action4 <- <{ p.Label(buffer[begin:end]) }> */
		nil,
	}
	p.rules = _rules
}
