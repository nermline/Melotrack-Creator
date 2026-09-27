// Presentation themes. The look is defined in show.css under [data-theme=…];
// this file only carries what the editor needs to show a picker.
export const THEMES = {
    neon: {
        id: 'neon',
        name: 'Неон',
        description: 'Синтвейв-вечірка: фіолетова ніч, рожеве й бірюзове світіння, сітка до горизонту.',
        swatch: 'linear-gradient(90deg, #ff2bd6, #7b5cff, #22e4ff)',
    },
    vinyl: {
        id: 'vinyl',
        name: 'Вініл',
        description: 'Теплий ретро-лаунж: кремові тони, платівка, що крутиться, і шрифт із засічками.',
        swatch: 'linear-gradient(90deg, #f28c28, #e0b43a, #f3e6cc)',
    },
    stage: {
        id: 'stage',
        name: 'Сцена',
        description: 'Концертний зал: чорна сцена, промені прожекторів і золоті акценти.',
        swatch: 'linear-gradient(90deg, #f5c542, #fff1b8, #f5c542)',
    },
};

export const THEME_LIST = Object.values(THEMES);
