# blueclip

my implementation of a clipboard manager for i3

I got inspiration from greenclip, but got disappointed due to some small problems.

- Unable to print selection "as is" so I could display a preview window
- Clipboard full of junk and no easy way to maintain it. like single character copies or meaningless words

So why not, let's implement it myself, how hard it can be?

I wanted something that

- Integrates with fzf
- Monitors clipboard and primary selection
- Allows me to preview multiple lines

So far I have

- [x] Integrates with fzf
- [x] Monitos clipboard, primary and secondary
- [x] supports copy targets UTF8_STRING x-special/gnome-copied-files and image/png
- [x] has a very nice cli tool to interact with it
- [x] Previews png images as ASCII art because I can
- [x] keeps my importan selections nearby

## Selection model

Instead of keeping a simple stack of selections, I've model it with 3 categories.

- Ephemeral selections
- Important selections
- Last selection

Last selection is whatever you just selected, Easy!. It will be always the first thing you see when you list your selections.

Ephemeral selections are everything you select by default. It has a capacity that if reached, it will drop old selections

Important selections are anything that you ever copied, The biggest advantage is that it has a different limit from Ephemeral selections, so it is likely to stay around for much longer.

When you list, you always get Last selection, but then you get a mix of Important/Ephemeral interlocked. Something like

```
Last Selection
Important
Ephemeral
Important
Ephemeral
Important
Ephemeral
Important
Ephemeral
```

Reasoning is that you may want to see what is in your current selection, that is why last selection is first thing. Then you are equally likely to be looking for an old important selection or a new recent selection. So both are interlocked.

In any case, you can always filter with fzf. but I hope this will make it quicker for the cases where I just scroll over it. (At least for myself)




