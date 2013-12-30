#include "clar_libgit2.h"
#include "posix.h"

void test_index_rename__single_file(void)
{
	git_repository *repo;
	git_index *index;
	size_t position;
	git_oid expected;
	const git_index_entry *entry;

	p_mkdir("rename", 0700);

	cl_git_pass(git_repository_init(&repo, "./rename", 0));
	cl_git_pass(git_repository_index(&index, repo));

	cl_assert(git_index_entrycount(index) == 0);

	cl_git_mkfile("./rename/lame.name.txt", "new_file\n");

	/* This should add a new blob to the object database in 'd4/fa8600b4f37d7516bef4816ae2c64dbf029e3a' */
	cl_git_pass(git_index_add_bypath(index, "lame.name.txt"));
	cl_assert(git_index_entrycount(index) == 1);

	cl_git_pass(git_oid_fromstr(&expected, "d4fa8600b4f37d7516bef4816ae2c64dbf029e3a"));

	cl_assert(!git_index_find(&position, index, "lame.name.txt"));

	entry = git_index_get_byindex(index, position);
	cl_assert(git_oid_cmp(&expected, &entry->oid) == 0);

	/* This removes the entry from the index, but not from the object database */
	cl_git_pass(git_index_remove(index, "lame.name.txt", 0));
	cl_assert(git_index_entrycount(index) == 0);

	p_rename("./rename/lame.name.txt", "./rename/fancy.name.txt");

	cl_git_pass(git_index_add_bypath(index, "fancy.name.txt"));
	cl_assert(git_index_entrycount(index) == 1);

	cl_assert(!git_index_find(&position, index, "fancy.name.txt"));

	entry = git_index_get_byindex(index, position);
	cl_assert(git_oid_cmp(&expected, &entry->oid) == 0);

	git_index_free(index);
	git_repository_free(repo);

	cl_fixture_cleanup("rename");
}
