# Measuring fidelity

This page describes `tools/fidelity`, which scores spine's slide
renders against a reference render of the same deck. It is for
contributors and users judging how close a render is to PowerPoint or
LibreOffice.
It is part of the [rendering guide](../rendering.md), which has the quick
start and the entry points.

`tools/fidelity` compares spine's slide renders with a reference render of the
same deck, such as PowerPoint's PNG export (File > Export, as PNG, every
slide) or LibreOffice's. Render at the reference's size, or let the tool
resample, then compare:

```sh
./spine-render -input deck.pptx -out ours -font 'Calibri=fonts/Calibri.ttf'
go run ./tools/fidelity -ours ours -ref reference -out report
```

Files pair by the last number in their names (`slide-0001.png` with
`Slide1.png`). Each slide gets its structural similarity (SSIM of luma over
8 by 8 windows; 1 is identical), mean absolute channel difference, the share
of pixels differing by more than `-threshold` (32 by default), and a diff
image of the faded reference with those pixels red. `report.md` and
`report.json` collect them. Fonts matter most: map the deck's fonts to the
files the reference used, or differences in text dominate.
