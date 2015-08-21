function! Tracker()
    let story = expand("<cword>")
    let url = "https://www.pivotaltracker.com/story/show/" . story
    silent exec "!open '" . url . "'" | redraw!
endfunction
nnoremap gs :call Tracker()<CR>
