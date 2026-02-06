package diagram

// --- Simple Value Types ---

// BoolVal represents a boolean value element (dgm:orgChart, dgm:bulletEnabled)
type BoolVal struct {
	Val bool `xml:"val,attr"`
}

// IntVal represents an integer value element (dgm:chMax, dgm:chPref)
type IntVal struct {
	Val int32 `xml:"val,attr"`
}

// DirVal represents CT_Direction (dgm:dir) - direction value
type DirVal struct {
	Val string `xml:"val,attr"` // ST_Direction: norm, rev
}

// AnimOneVal represents CT_AnimOne (dgm:animOne) - animation one value
type AnimOneVal struct {
	Val string `xml:"val,attr"` // ST_AnimOneStr: none, one, branch
}

// AnimLvlVal represents CT_AnimLvl (dgm:animLvl) - animation level value
type AnimLvlVal struct {
	Val string `xml:"val,attr"` // ST_AnimLvlStr: none, lvl, ctr
}

// ResizeHandlesVal represents CT_ResizeHandles (dgm:resizeHandles)
type ResizeHandlesVal struct {
	Val string `xml:"val,attr"` // ST_ResizeHandlesStr: exact, rel
}

// --- Enumeration Types (ST_*) ---

// ST_PtType values:
//   node      - Standard node
//   asst      - Assistant node
//   doc       - Document root
//   pres      - Presentation element
//   parTrans  - Parent transition
//   sibTrans  - Sibling transition

// ST_CxnType values:
//   parOf                - Parent-of relationship
//   presOf               - Presentation-of relationship
//   presParOf            - Presentation parent-of relationship
//   unknownRelationship  - Unknown relationship

// ST_AlgorithmType values:
//   composite  - Composite layout
//   conn       - Connector algorithm
//   cycle      - Cycle layout
//   hierChild  - Hierarchy child
//   hierRoot   - Hierarchy root
//   pyra       - Pyramid layout
//   lin        - Linear layout
//   sp         - Space algorithm
//   tx         - Text algorithm
//   snake      - Snake layout

// ST_ConstraintType values:
//   none, alignOff, begMarg, begPad, b, bMarg, bOff, bPad, connDist, ctrX,
//   ctrXOff, ctrY, ctrYOff, diam, endMarg, endPad, h, hArH, hOff, l, lMarg,
//   lOff, lPad, primFontSz, pyraAcctRatio, r, rMarg, rOff, rPad, secFontSz,
//   secSibSp, sibSp, sp, stemThick, t, tMarg, tOff, tPad, userA, userB,
//   userC, userD, userE, userF, userG, userH, userI, userJ, userK, userL,
//   userM, userN, userO, userP, userQ, userR, userS, userT, userU, userV,
//   userW, userX, userY, userZ, w, wArH, wOff

// ST_ConstraintRelationship values:
//   self, ch, des

// ST_ElementType values:
//   all, asst, doc, node, norm, nonAsst, nonNorm, parTrans, pres, sibTrans

// ST_FunctionType values:
//   cnt, pos, revPos, posEven, posOdd, var, depth, maxDepth

// ST_FunctionOperator values:
//   equ, neq, gt, lt, gte, lte

// ST_ParameterID values:
//   horzAlign, vertAlign, chDir, chAlign, secChAlign, linDir, secLinDir,
//   stElem, bendPt, connRout, begSty, endSty, dim, rotPath, ctrShpMap,
//   nodeHorzAlign, nodeVertAlign, fallback, txDir, pyraAcctPos, pyraAcctTxDef,
//   txBlDir, txAnchorHorz, txAnchorVert, txAnchorHorzCh, txAnchorVertCh,
//   parTxLTRAlign, parTxRTLAlign, shpTxLTRAlignCh, shpTxRTLAlignCh, autoTxRot,
//   grDir, flowDir, contDir, bkpt, off, hierAlign, bkPtFixedVal, stBulletLvl,
//   stAng, spanAng, ar, lnSpPar, lnSpAfParP, lnSpCh, lnSpAfChP, rtShortDist,
//   alignTx, pyraLvlNode, pyraAcctBkgdNode, pyraAcctTxNode, pyraLvlDel

// ST_ChildOrderType values:
//   b  - Bottom to top
//   t  - Top to bottom

// ST_ClrAppMethod values:
//   span    - Span across items
//   cycle   - Cycle through colors
//   repeat  - Repeat colors

// ST_HueDir values:
//   cw   - Clockwise
//   ccw  - Counter-clockwise

// ST_Direction values:
//   norm  - Normal direction
//   rev   - Reversed direction

// ST_AnimOneStr values:
//   none    - No animation
//   one     - Animate as one
//   branch  - Animate by branch

// ST_AnimLvlStr values:
//   none  - No animation levels
//   lvl   - By level
//   ctr   - From center

// ST_ResizeHandlesStr values:
//   exact  - Exact resize handles
//   rel    - Relative resize handles

// ST_AxisType values (comma-separated list in axis attribute):
//   self, ch, des, desOrSelf, par, ancst, ancstOrSelf, followSib, precedSib,
//   follow, preced, root, none
