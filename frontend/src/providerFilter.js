export function filterProviders(providers, category, query) {
  const search = query.trim().toLocaleLowerCase();
  return providers.filter((provider) => {
    const archived = provider.status === 'archived';
    const matchesCategory = category === 'all' ||
      (category === 'archived' ? archived : provider.category === category && !archived);
    return matchesCategory && [provider.name, provider.id].some((value) =>
      (value || '').toLocaleLowerCase().includes(search)
    );
  });
}
