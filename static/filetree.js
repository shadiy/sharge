// filetree.js: selection and context menu for file tree
let selectedItems = new Set();
let lastSelected = null;

function clearSelection() {
  document.querySelectorAll('.filetree-selected').forEach(el => el.classList.remove('filetree-selected'));
  selectedItems.clear();
}

function selectItem(li, multi = false) {
  if (!multi) clearSelection();
  li.classList.add('filetree-selected');
  selectedItems.add(li.dataset.path);
  lastSelected = li;
}

function toggleItem(li) {
  if (li.classList.contains('filetree-selected')) {
    li.classList.remove('filetree-selected');
    selectedItems.delete(li.dataset.path);
  } else {
    li.classList.add('filetree-selected');
    selectedItems.add(li.dataset.path);
    lastSelected = li;
  }
}

function getSelectedPaths() {
  return Array.from(selectedItems);
}

function updateActionButtons() {
  const openBtn = document.getElementById('filetree-open');
  const downloadBtn = document.getElementById('filetree-download');
  const renameBtn = document.getElementById('filetree-rename');
  const deleteBtn = document.getElementById('filetree-delete');
  const selected = getSelectedPaths();
  let canOpen = false;
  if (openBtn) {
    if (selected.length === 1) {
      let li = document.querySelector('.filetree-selected');
      // Only enable if not a folder (no details child)
      canOpen = li && !li.querySelector('details');
    }
    if (canOpen) {
      openBtn.href = `/view/${encodeURIComponent(selected[0])}`;
    } else {
      openBtn.href = '#';
    }

    setATagDisabled(openBtn, !canOpen);
  }

  if (downloadBtn) {
    setATagDisabled(downloadBtn, selected.length === 0);

    let path = selected.join("&f=");
    console.log(path);
    downloadBtn.href = `/dl?f=${path}`;
  }

  if (renameBtn) renameBtn.disabled = selected.length !== 1;
  if (deleteBtn) deleteBtn.disabled = selected.length === 0;
}

function setATagDisabled(tag, state) {
  if (state) {
    tag.style.pointerEvents = 'none';
    tag.style.opacity = '0.5';
  } else {
    tag.style.pointerEvents = '';
    tag.style.opacity = '';
  }
}

document.addEventListener('DOMContentLoaded', () => {
  document.querySelectorAll('li[data-path]').forEach(entry => {
    entry.addEventListener('click', e => {
      if (e.ctrlKey || e.metaKey) {
        toggleItem(entry);
      } else {
        selectItem(entry);
      }
      updateActionButtons();
      e.stopPropagation();
    });
  });


    const renameModal = document.getElementById('rename-modal');
    const renameModalPath = document.getElementById('rename-modal-path');
    const renameModalInput = document.getElementById('rename-modal-input');
    const renameModalCancel = document.getElementById('rename-modal-cancel');
    const renameModalApply = document.getElementById('rename-modal-apply');
    const mkdirModal = document.getElementById('mkdir-modal');
    const mkdirModalInput = document.getElementById('mkdir-modal-input');
    const mkdirModalCancel = document.getElementById('mkdir-modal-cancel');
    const mkdirModalApply = document.getElementById('mkdir-modal-apply');

    // Action buttons
    const renameBtn = document.getElementById('filetree-rename');
    const deleteBtn = document.getElementById('filetree-delete');
    const mkdirBtn = document.getElementById('filetree-mkdir');

    if (mkdirBtn) {
      mkdirBtn.addEventListener('click', () => {
        mkdirModalInput.value = '';
        mkdirModal.style.display = 'flex';
        mkdirModalInput.focus();
      });
    }

    mkdirModalCancel.addEventListener('click', () => {
      mkdirModal.style.display = 'none';
    });

    mkdirModalApply.addEventListener('click', () => {
      const dirName = mkdirModalInput.value.trim();
      if (!dirName) {
        addToast('Directory name cannot be empty.');
        return;
      }

      fetch(`/mkdir?d=${encodeURIComponent(dirName)}`, {method: 'POST'})
        .then(response => {
          if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
        })
        .then(() => {
          addToast(`Directory created successfully`);
          mkdirModal.style.display = 'none';
          window.location.reload();
        })
        .catch(error => {
          addToast(`Failed to create directory: ${error}`);
          mkdirModal.style.display = 'none';
        });
    });

    if (renameBtn) {
      renameBtn.addEventListener('click', () => {
        const selected = getSelectedPaths();
        if (selected.length !== 1) {
          addToast('Select exactly one file or folder to rename.');
          return;
        }
        const path = selected[0];
        renameModalPath.textContent = path;
        // Suggest only the filename for editing
        //let lastSlash = path.lastIndexOf('/') >= 0 ? path.lastIndexOf('/') : path.lastIndexOf('\\');
        //let filename = lastSlash >= 0 ? path.slice(lastSlash + 1) : path;
        renameModalInput.value = path;
        renameModalInput.select();
        renameModal.style.display = 'flex';
        renameModalInput.focus();
      });
    }

    renameModalCancel.addEventListener('click', () => {
      renameModal.style.display = 'none';
    });

    renameModalApply.addEventListener('click', () => {
      const selected = getSelectedPaths();
      if (selected.length !== 1) {
        addToast('Select exactly one file or folder to rename.');
        renameModal.style.display = 'none';
        return;
      }
      const oldPath = selected[0];
      //let lastSlash = oldPath.lastIndexOf('/') >= 0 ? oldPath.lastIndexOf('/') : oldPath.lastIndexOf('\\');
      //let dir = lastSlash >= 0 ? oldPath.slice(0, lastSlash + 1) : '';
      //const dir = selected[0];
      const newPath = renameModalInput.value.trim();
      if (!newPath) {
        addToast('New name cannot be empty.');
        return;
      }

      if (newPath === oldPath) {
        addToast('Name unchanged.');
        renameModal.style.display = 'none';
        return;
      }

      fetch(`/re?o=${encodeURIComponent(oldPath)}&n=${encodeURIComponent(newPath)}`, {method: 'POST'})
        .then(response => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
        })
        .then(() => {
          addToast(`Rename successful`);
          renameModal.style.display = 'none';
          // TODO: reload window or some other way to display rename
          window.location.reload();
        })
        .catch(error => {
          addToast(`Failed to rename ${error}`);
          renameModal.style.display = 'none';
        });
    });

    if (deleteBtn) {
      deleteBtn.addEventListener('click', () => {
        getSelectedPaths().forEach(path => {
          fetch(`/rm?f=${encodeURIComponent(path)}`, {method: 'POST'})
          .then(response => {
            if (!response.ok) {
              throw new Error(`HTTP error! status: ${response.status}`);
            }
          })
          .then(() => {
            addToast(`Delete successful`);
            document.querySelector(`li[data-path="${path}"]`).remove();
          })
          .catch(error => addToast(`Failed to delete ${error}`));
        });
      });
    }
    updateActionButtons();
});
